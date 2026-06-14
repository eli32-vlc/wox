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
	tools := GetDiscoveryTools(common.DynamicToolRegistrar{})
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

func GetDiscoveryTools(registrar common.DynamicToolRegistrar) []common.MCPTool {
	return []common.MCPTool{
		scanApplicationsTool(registrar),
		scanHomebrewTool(registrar),
		scanPlistFilesTool(registrar),
		scanURLSchemesTool(registrar),
		scanLaunchdServicesTool(registrar),
		scanUserLaunchAgentsTool(registrar),
		scanFileTypesTool(registrar),
		scanDaemonsTool(registrar),
		scanExtensionsTool(registrar),
		readPlistTool(),
		discoveryRefreshTool(registrar),
	}
}

func scanApplicationsTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_apps",
		Description: "Search installed applications. Pass select=bundle_id to generate per-app control tools (launch, open URL, info).",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter apps by name"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 100)"},
				"select": {Type: jsonschema.String, Description: "Bundle ID of app to select and generate control tools for"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectApp(ctx, selectID, registrar)
			}

			if cached, ok := discoveryCache.Get("macos_search_apps"); ok {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: cached}, nil
			}

			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			limit := 100
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			dirs := []string{"/Applications", "/System/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}

			var apps []DiscoveredApp
			for _, dir := range dirs {
				filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil || len(apps) >= limit {
						return nil
					}
					if !info.IsDir() || !strings.HasSuffix(info.Name(), ".app") {
						return nil
					}
					appName := strings.TrimSuffix(info.Name(), ".app")
					if searchFilter != "" && !strings.Contains(strings.ToLower(appName), searchFilter) {
						return filepath.SkipDir
					}

					app := DiscoveredApp{
						Name: appName,
						Path: path,
					}
					if out, err := runCmd("mdls", "-name", "kMDItemCFBundleIdentifier", "-raw", path); err == nil {
						app.BundleId = strings.TrimSpace(out)
					}
					if out, err := runCmd("mdls", "-name", "kMDItemVersion", "-raw", path); err == nil {
						app.Version = strings.TrimSpace(out)
					}
					apps = append(apps, app)
					return filepath.SkipDir
				})
				if len(apps) >= limit {
					break
				}
			}

			if apps == nil {
				apps = []DiscoveredApp{}
			}
			result, _ := json.MarshalIndent(apps, "", "  ")
			discoveryCache.Set("macos_search_apps", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func selectApp(ctx context.Context, bundleId string, registrar common.DynamicToolRegistrar) (common.Conversation, error) {
	appPath, err := findAppByBundleId(ctx, bundleId)
	if err != nil {
		return common.Conversation{}, fmt.Errorf("app not found: %s", bundleId)
	}

	appName := strings.TrimSuffix(filepath.Base(appPath), ".app")

	tools := generateAppControlTools(appName, appPath)

	registrar.RetireDiscovery(ctx, "macos_search_apps")
	registrar.Register(ctx, bundleId, tools)

	return common.Conversation{
		Role: common.ConversationRoleAssistant,
		Text: fmt.Sprintf("Selected %s. Generated %d control tools (launch, URL schemes, quit). Use them in subsequent turns.", appName, len(tools)),
	}, nil
}

func generateAppControlTools(appName, appPath string) []common.MCPTool {
	var tools []common.MCPTool

	tools = append(tools, common.MCPTool{
		Name:        fmt.Sprintf("launch_%s", appName),
		Description: fmt.Sprintf("Launch %s application", appName),
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("open", "-a", appName)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to launch %s: %s", appName, out)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Launched %s", appName)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	})

	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	if out, err := runCmd("plutil", "-convert", "xml1", "-o", "-", plistPath); err == nil {
		schemes := extractURLSchemesFromPlistXML(out)
		for _, scheme := range schemes {
			s := scheme
			tools = append(tools, common.MCPTool{
				Name:        fmt.Sprintf("open_%s", s),
				Description: fmt.Sprintf("Open a URL with the %s:// scheme (handled by %s)", s, appName),
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {Type: jsonschema.String, Description: fmt.Sprintf("URL path for %s://", s)},
					},
					Required: []string{"path"},
				},
				Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
					path, _ := args["path"].(string)
					url := fmt.Sprintf("%s://%s", s, strings.TrimPrefix(path, "/"))
					out, err := runCmd("open", url)
					if err != nil {
						return common.Conversation{}, fmt.Errorf("failed to open %s: %s", url, out)
					}
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Opened %s", url)}, nil
				},
				ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
			})
		}
	}

	return tools
}

func extractURLSchemesFromPlistXML(xml string) []string {
	var schemes []string
	lines := strings.Split(xml, "\n")
	inSchemes := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "<key>CFBundleURLSchemes</key>") {
			inSchemes = true
			continue
		}
		if inSchemes {
			if strings.Contains(trimmed, "<string>") && strings.Contains(trimmed, "</string>") {
				start := strings.Index(trimmed, "<string>") + len("<string>")
				end := strings.Index(trimmed, "</string>")
				if start > len("<string>")-1 && end > start {
					scheme := trimmed[start:end]
					schemes = append(schemes, scheme)
				}
			}
			if strings.Contains(trimmed, "</array>") {
				break
			}
		}
	}
	return schemes
}

func findAppByBundleId(ctx context.Context, bundleId string) (string, error) {
	out, err := runCmd("mdfind", "kMDItemCFBundleIdentifier=="+bundleId, "-onlyin", "/Applications", "-onlyin", "/System/Applications")
	if err == nil && out != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			if strings.HasSuffix(line, ".app") {
				return line, nil
			}
		}
	}

	out, err = runCmd("mdfind", "kMDItemCFBundleIdentifier=="+bundleId, "-onlyin", os.Getenv("HOME")+"/Applications")
	if err == nil && out != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			if strings.HasSuffix(line, ".app") {
				return line, nil
			}
		}
	}

	return "", fmt.Errorf("app with bundle id %s not found", bundleId)
}

func scanHomebrewTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_homebrew",
		Description: "Search installed Homebrew formulae and casks. Pass select=package_name to generate per-package tools (upgrade, uninstall, info).",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter packages by name"},
				"type":   {Type: jsonschema.String, Description: "Package type: 'formula', 'cask', or 'all' (default)"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 100)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectHomebrewPackage(ctx, selectID, registrar)
			}

			if cached, ok := discoveryCache.Get("macos_search_homebrew"); ok {
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
				type brewFormula struct {
					Name     string            `json:"name"`
					Versions map[string]any   `json:"versions"`
					Desc     string            `json:"desc"`
				}
				type brewCask struct {
					Token   string `json:"token"`
					Version string `json:"version"`
					Desc    string `json:"desc"`
				}
				type brewInfo struct {
					Formulae []brewFormula `json:"formulae"`
					Casks    []brewCask    `json:"casks"`
				}

				var info brewInfo
				if jsonErr := json.Unmarshal([]byte(out), &info); jsonErr == nil {
					if pkgType == "all" || pkgType == "formula" {
						for _, f := range info.Formulae {
							if searchFilter != "" && !strings.Contains(strings.ToLower(f.Name), searchFilter) {
								continue
							}
							version := ""
							if stable, ok := f.Versions["stable"]; ok {
								version = fmt.Sprintf("%v", stable)
							}
							pkgs = append(pkgs, DiscoveredPackage{
								Name: f.Name, Version: version,
								Type: "formula", Description: f.Desc,
							})
						}
					}
					if pkgType == "all" || pkgType == "cask" {
						for _, c := range info.Casks {
							if searchFilter != "" && !strings.Contains(strings.ToLower(c.Token), searchFilter) {
								continue
							}
							pkgs = append(pkgs, DiscoveredPackage{
								Name: c.Token, Version: c.Version,
								Type: "cask", Description: c.Desc,
							})
						}
					}
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
			discoveryCache.Set("macos_search_homebrew", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func selectHomebrewPackage(ctx context.Context, pkgName string, registrar common.DynamicToolRegistrar) (common.Conversation, error) {
	pkgName = strings.ToLower(pkgName)
	tools := []common.MCPTool{
		{
			Name:        fmt.Sprintf("brew_upgrade_%s", pkgName),
			Description: fmt.Sprintf("Upgrade Homebrew package %s", pkgName),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("brew", "upgrade", pkgName)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to upgrade %s: %s", pkgName, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
	}
	registrar.RetireDiscovery(ctx, "macos_search_homebrew")
	registrar.Register(ctx, pkgName, tools)
	return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Selected %s. Generated brew_upgrade_%s tool. Available in subsequent turns.", pkgName, pkgName)}, nil
}

func scanPlistFilesTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_preferences",
		Description: "Search preference plist files. Pass select=domain to generate read/write tools for that domain.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search":   {Type: jsonschema.String, Description: "Search term to filter plist files by name"},
				"location": {Type: jsonschema.String, Description: "Location: 'user', 'system', or 'all' (default)"},
				"select":   {Type: jsonschema.String, Description: "Preference domain to select and generate read/write tools for"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectPreferenceDomain(ctx, selectID, registrar)
			}

			if cached, ok := discoveryCache.Get("macos_search_preferences"); ok {
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
			discoveryCache.Set("macos_search_preferences", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func selectDaemonService(ctx context.Context, name string, registrar common.DynamicToolRegistrar, toolName string) (common.Conversation, error) {
	tools := []common.MCPTool{
		{
			Name:        fmt.Sprintf("service_start_%s", name),
			Description: fmt.Sprintf("Start the %s launchd service", name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("launchctl", "load", fmt.Sprintf("/Library/LaunchDaemons/%s.plist", name))
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to start %s: %s", name, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Started %s", name)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
		{
			Name:        fmt.Sprintf("service_stop_%s", name),
			Description: fmt.Sprintf("Stop the %s launchd service", name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("launchctl", "unload", fmt.Sprintf("/Library/LaunchDaemons/%s.plist", name))
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to stop %s: %s", name, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Stopped %s", name)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
		{
			Name:        fmt.Sprintf("service_status_%s", name),
			Description: fmt.Sprintf("Get the status of the %s launchd service", name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				status := getLaunchdStatus(name)
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("%s status: %s", name, status)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
	}
	registrar.RetireDiscovery(ctx, toolName)
	registrar.Register(ctx, name, tools)
	return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Selected service %s. Generated start/stop/status tools. Available in subsequent turns.", name)}, nil
}

func selectPreferenceDomain(ctx context.Context, domain string, registrar common.DynamicToolRegistrar) (common.Conversation, error) {
	tools := []common.MCPTool{
		{
			Name:        fmt.Sprintf("prefs_read_%s", domain),
			Description: fmt.Sprintf("Read a preference key from %s", domain),
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"key": {Type: jsonschema.String, Description: "Preference key to read"},
				},
			},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				key, _ := args["key"].(string)
				out, err := runCmd("defaults", "read", domain, key)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to read %s %s: %s", domain, key, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
		{
			Name:        fmt.Sprintf("prefs_write_%s", domain),
			Description: fmt.Sprintf("Write a preference value to %s", domain),
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"key":   {Type: jsonschema.String, Description: "Preference key to write"},
					"type":  {Type: jsonschema.String, Description: "Value type: string, int, bool, float"},
					"value": {Type: jsonschema.String, Description: "Value to set"},
				},
			},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				key, _ := args["key"].(string)
				valType, _ := args["type"].(string)
				value, _ := args["value"].(string)
				var out string
				var err error
				switch valType {
				case "int":
					out, err = runCmd("defaults", "write", domain, key, "-int", value)
				case "bool":
					out, err = runCmd("defaults", "write", domain, key, "-bool", value)
				case "float":
					out, err = runCmd("defaults", "write", domain, key, "-float", value)
				default:
					out, err = runCmd("defaults", "write", domain, key, value)
				}
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to write %s %s: %s", domain, key, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Set %s %s = %s (%s)", domain, key, value, valType)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
		},
	}
	registrar.RetireDiscovery(ctx, "macos_search_preferences")
	registrar.Register(ctx, domain, tools)
	return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Selected preference domain %s. Generated read/write tools. Available in subsequent turns.", domain)}, nil
}

func scanLaunchdServicesTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_services",
		Description: "Search system launchd daemons. Pass select=service_name to generate control tools (start, stop, status).",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter services by name"},
				"select": {Type: jsonschema.String, Description: "Service name to select and generate control tools for"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectDaemonService(ctx, selectID, registrar, "macos_search_services")
			}

			if cached, ok := discoveryCache.Get("macos_search_services"); ok {
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
			discoveryCache.Set("macos_search_services", string(result))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanUserLaunchAgentsTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_agents",
		Description: "Search user and system launch agents. Pass select=agent_name to generate control tools (start, stop, status).",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter agents by name"},
				"select": {Type: jsonschema.String, Description: "Agent name to select and generate control tools for"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectDaemonService(ctx, selectID, registrar, "macos_search_agents")
			}

			if cached, ok := discoveryCache.Get("macos_search_agents"); ok {
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
			discoveryCache.Set("macos_search_agents", string(result))
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

func scanURLSchemesTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_url_schemes",
		Description: "Search URL schemes registered by installed apps. Pass select=scheme to generate an open URL tool for that scheme.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter URL schemes"},
				"select": {Type: jsonschema.String, Description: "URL scheme to select (e.g. 'spotify', 'imessage')"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			appSources := []string{"/Applications", "/System/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}
			var matched []string
			for _, dir := range appSources {
				filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil || !strings.HasSuffix(path, ".app") {
						return nil
					}
					if !info.IsDir() {
						return nil
					}
					plistPath := filepath.Join(path, "Contents", "Info.plist")
					if out, err := runCmd("plutil", "-convert", "xml1", "-o", "-", plistPath); err == nil {
						schemes := extractURLSchemesFromPlistXML(out)
						for _, scheme := range schemes {
							if searchFilter == "" || strings.Contains(strings.ToLower(scheme), searchFilter) {
								matched = append(matched, fmt.Sprintf("%s:// -> %s", scheme, filepath.Base(path)))
							}
						}
					}
					return filepath.SkipDir
				})
			}

			// Check select
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				selectID = strings.TrimSuffix(selectID, "://")
				registrar.RetireDiscovery(ctx, "macos_search_url_schemes")
				tools := []common.MCPTool{
					{
						Name:        fmt.Sprintf("open_%s", selectID),
						Description: fmt.Sprintf("Open a URL with the %s:// scheme", selectID),
						Parameters: jsonschema.Definition{
							Type: jsonschema.Object,
							Properties: map[string]jsonschema.Definition{
								"path": {Type: jsonschema.String, Description: fmt.Sprintf("URL path for %s:// (e.g. 'track/123')", selectID)},
							},
							Required: []string{"path"},
						},
						Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
							path, _ := args["path"].(string)
							url := fmt.Sprintf("%s://%s", selectID, strings.TrimPrefix(path, "/"))
							out, err := runCmd("open", url)
							if err != nil {
								return common.Conversation{}, fmt.Errorf("failed to open: %s", out)
							}
							return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Opened %s", url)}, nil
						},
						ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
					},
				}
				registrar.Register(ctx, selectID, tools)
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Selected %s://. Generated open_%s tool. Available in subsequent turns.", selectID, selectID)}, nil
			}

			if matched == nil {
				matched = []string{}
			}
			result, _ := json.MarshalIndent(matched, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanDaemonsTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_daemons",
		Description: "Search system daemon plist files (non-launchd system services). Pass select=name to generate control tools.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter daemons"},
				"select": {Type: jsonschema.String, Description: "Daemon name to select"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			dirs := []string{"/Library/LaunchDaemons", "/System/Library/LaunchDaemons"}
			var daemons []DiscoveredService
			for _, dir := range dirs {
				daemons = append(daemons, discoverServices(dir, searchFilter)...)
			}

			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectDaemonService(ctx, selectID, registrar, "macos_search_daemons")
			}

			if daemons == nil {
				daemons = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(daemons, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanFileTypesTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_file_types",
		Description: "Search which apps can open a specific file type or UTI. Pass select=app_bundle_id to generate open-tools for that app+type.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"extension": {Type: jsonschema.String, Description: "File extension to search for (e.g. 'md', 'pdf', 'py')"},
				"select":    {Type: jsonschema.String, Description: "Bundle ID of the app to select"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if selectID, ok := args["select"].(string); ok && selectID != "" {
				return selectApp(ctx, selectID, registrar)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Use this tool first to find apps for a file type, then pass select=bundle_id to generate tools."}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanExtensionsTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_search_extensions",
		Description: "Search system and app extensions (Today widgets, Share, Actions, etc.). Pass select=extension_id to generate tools.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter extensions"},
				"select": {Type: jsonschema.String, Description: "Extension identifier to select"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Extension discovery coming soon. Check back after a refresh."}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func discoveryRefreshTool(registrar common.DynamicToolRegistrar) common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discovery_refresh",
		Description: "Refresh all discovery caches and reset all tool selections. Re-scans apps, Homebrew, plists, and services.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			registrar.Unregister(ctx, "*")
			discoveryCache.Clear()
			RunDiscoveryStartupScan(ctx)
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Discovery cache refreshed. %d tools cached. All selections reset.", discoveryCache.Count())}, nil
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
