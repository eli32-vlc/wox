package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

var discoveryCache = &DiscoveryCache{data: make(map[string]string)}

type DiscoveryCache struct {
	mu       sync.RWMutex
	data     map[string]string
	lastScan int64
}

func (c *DiscoveryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *DiscoveryCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.lastScan = time.Now().Unix()
}

func (c *DiscoveryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]string)
	c.lastScan = 0
}

func (c *DiscoveryCache) Age() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastScan == 0 {
		return 0
	}
	return time.Duration(time.Now().Unix()-c.lastScan) * time.Second
}

func (c *DiscoveryCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

func RunDiscoveryStartupScan(ctx context.Context) {
	tools := GetDiscoveryTools()
	for _, tool := range tools {
		result, err := tool.Callback(ctx, nil)
		if err == nil && result.Text != "" {
			discoveryCache.Set(tool.Name, result.Text)
		}
	}
}

type DiscoveredApp struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	BundleId string `json:"bundleId,omitempty"`
	Version  string `json:"version,omitempty"`
}

type DiscoveredPackage struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type DiscoveredService struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	PlistPath   string `json:"plistPath,omitempty"`
	Description string `json:"description,omitempty"`
}

func GetDiscoveryTools() []common.MCPTool {
	return []common.MCPTool{
		scanApplicationsTool(),
		scanHomebrewTool(),
		scanPlistFilesTool(),
		scanLaunchdServicesTool(),
		scanUserLaunchAgentsTool(),
		readPlistTool(),
		discoveryRefreshTool(),
	}
}

func scanApplicationsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_apps",
		Description: "Scan /Applications and ~/Applications directories for installed applications. Uses Spotlight (mdfind) for fast indexing and mdls for metadata.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter apps by name"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 50)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if cached, ok := discoveryCache.Get("macos_discover_apps"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			limit := 50
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			var apps []DiscoveredApp
			dirs := []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}

			for _, dir := range dirs {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, entry := range entries {
					if !strings.HasSuffix(entry.Name(), ".app") {
						continue
					}
					appName := strings.TrimSuffix(entry.Name(), ".app")
					if searchFilter != "" && !strings.Contains(strings.ToLower(appName), searchFilter) {
						continue
					}
					appPath := filepath.Join(dir, entry.Name())

					app := DiscoveredApp{
						Name: appName,
						Path: appPath,
					}

					if out, err := runCmd("mdls", "-name", "kMDItemCFBundleIdentifier", "-raw", appPath); err == nil {
						app.BundleId = strings.TrimSpace(out)
					}
					if out, err := runCmd("mdls", "-name", "kMDItemVersion", "-raw", appPath); err == nil {
						app.Version = strings.TrimSpace(out)
					}

					apps = append(apps, app)
					if len(apps) >= limit {
						break
					}
				}
				if len(apps) >= limit {
					break
				}
			}

			if apps == nil {
				apps = []DiscoveredApp{}
			}
			result, _ := json.MarshalIndent(apps, "", "  ")
			discoveryCache.Set("macos_discover_apps", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanHomebrewTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_homebrew",
		Description: "Scan Homebrew for installed formulae and casks using brew info JSON output for rich metadata. Requires Homebrew to be installed.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter packages by name"},
				"type":   {Type: jsonschema.String, Description: "Package type: 'formula', 'cask', or 'all' (default)"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 100)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if cached, ok := discoveryCache.Get("macos_discover_homebrew"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			pkgType := "all"
			if t, ok := args["type"].(string); ok {
				pkgType = t
			}
			limit := 100
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			var pkgs []DiscoveredPackage

			// Try JSON output first (richer data)
			out, err := runCmd("brew", "info", "--json=v2", "--installed")
			if err == nil {
				// Parse JSON: brew info --json=v2 returns {"formulae": [...], "casks": [...]}
				// Simple parsing of name/token, version, desc fields
				parseBrewJSON := func(data string, ptype string) {
					lines := strings.Split(data, "\n")
					var currentName, currentVersion, currentDesc string
					inCasks := false
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(line, "\"casks\"") {
							inCasks = true
							currentName = ""
							continue
						}
						if ptype == "formula" && inCasks {
							break
						}
						if ptype == "cask" && !inCasks {
							continue
						}
						// Extract name/token
						if strings.Contains(line, "\"full_token\"") || strings.Contains(line, "\"token\"") || strings.Contains(line, "\"name\"") {
							currentName = extractJSONStringField(line)
						}
						if strings.Contains(line, "\"versioned_formulae\"") { // version follows
							if idx := strings.Index(line, ":"); idx != -1 {
								currentVersion = strings.TrimSpace(line[:idx])
							}
						}
						if strings.Contains(line, "\"version\"") && !strings.Contains(line, "\"versioned\"") {
							currentVersion = extractJSONStringField(line)
						}
						if strings.Contains(line, "\"desc\"") {
							currentDesc = extractJSONStringField(line)
							if currentName != "" {
								if searchFilter == "" || strings.Contains(strings.ToLower(currentName), searchFilter) {
									pkgs = append(pkgs, DiscoveredPackage{
										Name: currentName, Version: currentVersion,
										Type: ptype, Description: currentDesc,
									})
								}
								currentName = ""
							}
						}
					}
				}
				if pkgType == "all" || pkgType == "formula" {
					parseBrewJSON(out, "formula")
				}
				if pkgType == "all" || pkgType == "cask" {
					parseBrewJSON(out, "cask")
				}
				if len(pkgs) > 0 {
					goto done
				}
			}

			// Fallback to list --versions
			if pkgType == "all" || pkgType == "formula" {
				out, err := runCmd("brew", "list", "--formula", "--versions")
				if err == nil {
					for _, line := range strings.Split(out, "\n") {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						parts := strings.Fields(line)
						if len(parts) > 0 {
							name := parts[0]
							version := ""
							if len(parts) > 1 {
								version = parts[1]
							}
							if searchFilter != "" && !strings.Contains(strings.ToLower(name), searchFilter) {
								continue
							}
							pkgs = append(pkgs, DiscoveredPackage{Name: name, Version: version, Type: "formula"})
						}
					}
				}
			}
			if pkgType == "all" || pkgType == "cask" {
				out, err := runCmd("brew", "list", "--cask", "--versions")
				if err == nil {
					for _, line := range strings.Split(out, "\n") {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						parts := strings.Fields(line)
						if len(parts) > 0 {
							name := parts[0]
							version := ""
							if len(parts) > 1 {
								version = parts[1]
							}
							if searchFilter != "" && !strings.Contains(strings.ToLower(name), searchFilter) {
								continue
							}
							pkgs = append(pkgs, DiscoveredPackage{Name: name, Version: version, Type: "cask"})
						}
					}
				}
			}

		done:
			if pkgs == nil {
				pkgs = []DiscoveredPackage{}
			}
			if len(pkgs) > limit {
				pkgs = pkgs[:limit]
			}
			result, _ := json.MarshalIndent(pkgs, "", "  ")
			discoveryCache.Set("macos_discover_homebrew", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanPlistFilesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_plists",
		Description: "Scan plist files in common macOS preference directories. Returns file paths and sizes. Use system_plist_read to read the content of a specific plist.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search":   {Type: jsonschema.String, Description: "Optional search term to filter plist files by name"},
				"location": {Type: jsonschema.String, Description: "Location: 'user' (~/Library/Preferences), 'system' (/Library/Preferences), or 'all' (default)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if cached, ok := discoveryCache.Get("macos_discover_plists"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			location := "all"
			if l, ok := args["location"].(string); ok {
				location = l
			}

			type PlistEntry struct {
				Path     string `json:"path"`
				File     string `json:"file"`
				Location string `json:"location"`
				SizeKB   int64  `json:"sizeKB"`
			}

			var entries []PlistEntry
			dirs := map[string]string{}

			if location == "all" || location == "user" {
				dirs["user"] = filepath.Join(os.Getenv("HOME"), "Library", "Preferences")
			}
			if location == "all" || location == "system" {
				dirs["system"] = "/Library/Preferences"
			}

			for loc, dir := range dirs {
				de, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range de {
					if !strings.HasSuffix(e.Name(), ".plist") {
						continue
					}
					if searchFilter != "" && !strings.Contains(strings.ToLower(e.Name()), searchFilter) {
						continue
					}
					info, _ := e.Info()
					var size int64
					if info != nil {
						size = info.Size() / 1024
					}
					entries = append(entries, PlistEntry{
						Path:     filepath.Join(dir, e.Name()),
						File:     e.Name(),
						Location: loc,
						SizeKB:   size,
					})
				}
			}

			if entries == nil {
				entries = []PlistEntry{}
			}
			result, _ := json.MarshalIndent(entries, "", "  ")
			discoveryCache.Set("macos_discover_plists", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanLaunchdServicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_services",
		Description: "Scan system-wide launchd daemons. Reads service names and running status from launchctl in a single pass.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter services by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if cached, ok := discoveryCache.Get("macos_discover_services"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			services := discoverServices("/Library/LaunchDaemons", searchFilter)
			if services == nil {
				services = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(services, "", "  ")
			discoveryCache.Set("macos_discover_services", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanUserLaunchAgentsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_agents",
		Description: "Scan user and system launch agents (~/Library/LaunchAgents and /Library/LaunchAgents). Reads names and status in a single pass.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter agents by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if cached, ok := discoveryCache.Get("macos_discover_agents"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			userDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
			combined := discoverServices(userDir, searchFilter)
			sysServices := discoverServices("/Library/LaunchAgents", searchFilter)
			combined = append(combined, sysServices...)

			if combined == nil {
				combined = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(combined, "", "  ")
			discoveryCache.Set("macos_discover_agents", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func readPlistTool() common.MCPTool {
	return common.MCPTool{
		Name:        "system_plist_read",
		Description: "Read the contents of a plist file. Handles both XML and binary plists by using plutil to convert. Returns the plist content as XML.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Full path to the plist file, e.g. '~/Library/Preferences/com.apple.finder.plist'"},
				"key":  {Type: jsonschema.String, Description: "Optional specific key to extract (e.g. 'CFBundleIdentifier')"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return common.Conversation{}, fmt.Errorf("path is required")
			}
			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(os.Getenv("HOME"), path[2:])
			}

			key, _ := args["key"].(string)

			var out string
			var err error

			if key != "" {
				// PlistBuddy handles both XML and binary plists
				out, err = runCmd("/usr/libexec/PlistBuddy", "-c", fmt.Sprintf("Print :%s", key), path)
				if err != nil {
					// Try defaults read as fallback
					domain := strings.TrimSuffix(filepath.Base(path), ".plist")
					out, err = runCmd("defaults", "read", domain, key)
				}
			} else {
				// Convert to XML regardless of original format
				out, err = runCmd("plutil", "-convert", "xml1", "-o", "-", path)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to read plist %s: %s", path, err.Error())
				}
			}

			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read plist %s key %s: %s", path, key, err.Error())
			}

			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func discoverServices(dir string, searchFilter string) []DiscoveredService {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var services []DiscoveredService
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".plist") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".plist")
		if searchFilter != "" && !strings.Contains(strings.ToLower(name), searchFilter) {
			continue
		}
		services = append(services, DiscoveredService{
			Name:      name,
			Status:    getLaunchdStatus(name),
			PlistPath: filepath.Join(dir, entry.Name()),
		})
	}
	return services
}

func getLaunchdStatus(label string) string {
	out, err := runCmd("launchctl", "print", fmt.Sprintf("system/%s", label))
	if err != nil {
		// Try user domain
		out, err = runCmd("launchctl", "print", fmt.Sprintf("gui/%d/%s", os.Getuid(), label))
	}
	if err != nil {
		return "unknown"
	}
	if strings.Contains(out, "state = running") {
		return "running"
	}
	return "stopped"
}

func extractJSONStringField(line string) string {
	// Extract string value between quotes after the colon
	idx := strings.Index(line, "\"")
	if idx == -1 {
		return ""
	}
	rest := line[idx+1:]
	endIdx := strings.LastIndex(rest, "\"")
	if endIdx == -1 {
		return rest
	}
	val := rest[:endIdx]
	// Remove trailing comma
	val = strings.TrimSuffix(val, ",")
	val = strings.Trim(val, "\"")
	return val
}

func discoveryRefreshTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discovery_refresh",
		Description: "Refresh all discovery caches. Re-scans applications, Homebrew packages, plists, and services.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			discoveryCache.Clear()
			RunDiscoveryStartupScan(ctx)
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Discovery cache refreshed. %d tools cached.", discoveryCache.Count())}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func init() {
	_, brewErr := exec.LookPath("brew")
	if brewErr != nil {
		fmt.Fprintf(os.Stderr, "macOS discovery: Homebrew not found, brew tools will not work\n")
	}
}
