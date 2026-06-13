package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

type DiscoveredApp struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	BundleId string `json:"bundleId,omitempty"`
	Version  string `json:"version,omitempty"`
}

type DiscoveredPackage struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"` // "formula" or "cask"
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
	}
}

func scanApplicationsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_apps",
		Description: "Scan /Applications and ~/Applications directories for installed applications. Returns app names, paths, bundle IDs, and versions.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter apps by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			dirs := []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}
			var apps []DiscoveredApp

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

					app := DiscoveredApp{
						Name: appName,
						Path: filepath.Join(dir, entry.Name()),
					}

					// Try to read Info.plist for bundle ID and version
					plistPath := filepath.Join(dir, entry.Name(), "Contents", "Info.plist")
					if info, err := os.ReadFile(plistPath); err == nil {
						content := string(info)
						// Simple parsing for common plist keys
						if id := extractPlistValue(content, "CFBundleIdentifier"); id != "" {
							app.BundleId = id
						}
						if ver := extractPlistValue(content, "CFBundleShortVersionString"); ver != "" {
							app.Version = ver
						}
						if app.BundleId == "" {
							if id := extractPlistValue(content, "CFBundleName"); id != "" {
								app.BundleId = id
							}
						}
					}

					apps = append(apps, app)
				}
			}

			result, _ := json.MarshalIndent(apps, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanHomebrewTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_homebrew",
		Description: "Scan Homebrew for installed formulae and casks. Requires Homebrew to be installed.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter packages by name"},
				"type":   {Type: jsonschema.String, Description: "Package type: 'formula', 'cask', or 'all' (default)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			pkgType := "all"
			if t, ok := args["type"].(string); ok {
				pkgType = t
			}

			var pkgs []DiscoveredPackage

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

			if pkgs == nil {
				pkgs = []DiscoveredPackage{}
			}
			result, _ := json.MarshalIndent(pkgs, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanPlistFilesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_plists",
		Description: "Scan plist files in common macOS preference and configuration directories",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search":   {Type: jsonschema.String, Description: "Optional search term to filter plist files by name"},
				"location": {Type: jsonschema.String, Description: "Location: 'user' (~/Library/Preferences), 'system' (/Library/Preferences), 'global' (/Library/Managed Preferences), or 'all' (default)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}
			location := "all"
			if l, ok := args["location"].(string); ok {
				location = l
			}

			dirs := map[string]string{}
			if location == "all" || location == "user" {
				dirs["user"] = filepath.Join(os.Getenv("HOME"), "Library", "Preferences")
			}
			if location == "all" || location == "system" {
				dirs["system"] = "/Library/Preferences"
			}
			if location == "all" || location == "global" {
				dirs["global"] = "/Library/Managed Preferences"
			}

			type PlistEntry struct {
				Path     string `json:"path"`
				File     string `json:"file"`
				Location string `json:"location"`
				SizeKB   int64  `json:"sizeKB"`
			}

			var entries []PlistEntry
			for loc, dir := range dirs {
				filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if info.IsDir() && path != dir {
						return filepath.SkipDir
					}
					if !strings.HasSuffix(info.Name(), ".plist") {
						return nil
					}
					if searchFilter != "" && !strings.Contains(strings.ToLower(info.Name()), searchFilter) {
						return nil
					}
					entries = append(entries, PlistEntry{
						Path:     path,
						File:     info.Name(),
						Location: loc,
						SizeKB:   info.Size() / 1024,
					})
					return nil
				})
			}

			if entries == nil {
				entries = []PlistEntry{}
			}
			result, _ := json.MarshalIndent(entries, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanLaunchdServicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_services",
		Description: "Scan system-wide launchd daemons in /Library/LaunchDaemons",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter services by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			return scanPlistDir("/Library/LaunchDaemons", searchFilter)
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanUserLaunchAgentsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_discover_agents",
		Description: "Scan user launch agents in ~/Library/LaunchAgents and /Library/LaunchAgents",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Optional search term to filter agents by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = strings.ToLower(s)
			}

			userDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
			userResult, _ := scanPlistDirRaw(userDir, searchFilter)
			systemResult, _ := scanPlistDirRaw("/Library/LaunchAgents", searchFilter)

			combined := []DiscoveredService{}
			combined = append(combined, userResult...)
			combined = append(combined, systemResult...)

			result, _ := json.MarshalIndent(combined, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_discovery"},
	}
}

func scanPlistDir(dir string, searchFilter string) (common.Conversation, error) {
	services, err := scanPlistDirRaw(dir, searchFilter)
	if err != nil {
		return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Error scanning %s: %s", dir, err.Error())}, nil
	}
	if services == nil {
		services = []DiscoveredService{}
	}
	result, _ := json.MarshalIndent(services, "", "  ")
	return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
}

func scanPlistDirRaw(dir string, searchFilter string) ([]DiscoveredService, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []DiscoveredService{}, nil
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

		svc := DiscoveredService{
			Name:      name,
			Status:    "unknown",
			PlistPath: filepath.Join(dir, entry.Name()),
		}

		// Try to get service status from launchctl
		if out, err := runCmd("launchctl", "list"); err == nil {
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, name) {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						if fields[0] == "-" {
							svc.Status = "stopped"
						} else {
							svc.Status = "running (PID: " + fields[0] + ")"
						}
					}
					break
				}
			}
		}

		services = append(services, svc)
	}

	return services, nil
}

// extractPlistValue does a simple key-value extraction from plist XML content.
// This is intentionally basic - full plist parsing would require additional dependencies.
func extractPlistValue(content string, key string) string {
	// Look for <key>CFBundleIdentifier</key><string>com.example.app</string>
	searchKey := "<key>" + key + "</key>"
	idx := strings.Index(content, searchKey)
	if idx == -1 {
		return ""
	}

	rest := content[idx+len(searchKey):]
	// Trim whitespace
	rest = strings.TrimSpace(rest)

	// Look for <string>...</string>
	if strings.HasPrefix(rest, "<string>") {
		endIdx := strings.Index(rest, "</string>")
		if endIdx != -1 {
			return rest[8:endIdx]
		}
	}

	return ""
}

func init() {
	// Register discovery tools
	// Check if Homebrew is available
	_, brewErr := exec.LookPath("brew")
	if brewErr != nil {
		fmt.Fprintf(os.Stderr, "macOS discovery: Homebrew not found, brew tools will not work\n")
	}
}
