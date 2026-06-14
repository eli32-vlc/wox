package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/plugin"

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
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type DiscoveredService struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	PlistPath   string `json:"plistPath,omitempty"`
	Description string `json:"description,omitempty"`
}

type PrefEntry struct {
	Path     string `json:"path"`
	File     string `json:"file"`
	Location string `json:"location"`
	SizeKB   int64  `json:"sizeKB"`
}

type SchemeEntry struct {
	Scheme  string `json:"scheme"`
	AppName string `json:"appName"`
	AppPath string `json:"appPath"`
}

func refreshPerItemTools(ctx context.Context) {
	// No-op: per-item tools are now generic and search tools scan on demand
}

// ------- Scan functions (no registrar, no hub) -------

func scanInstalledApps(ctx context.Context, searchFilter string, limit int) []DiscoveredApp {
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
	return apps
}

func scanHomebrew(ctx context.Context, searchFilter string, pkgType string, limit int) []DiscoveredPackage {
	var pkgs []DiscoveredPackage

	out, err := runCmd("brew", "info", "--json=v2", "--installed")
	if err == nil {
		type brewFormula struct {
			Name     string         `json:"name"`
			Versions map[string]any `json:"versions"`
			Desc     string         `json:"desc"`
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
			goto brewDone
		}
	}

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

brewDone:
	if len(pkgs) > limit {
		pkgs = pkgs[:limit]
	}
	return pkgs
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

func scanPreferenceDomains(ctx context.Context, searchFilter string, location string) []PrefEntry {
	type plistEntry struct {
		Path     string `json:"path"`
		File     string `json:"file"`
		Location string `json:"location"`
		SizeKB   int64  `json:"sizeKB"`
	}

	var entries []PrefEntry
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
			entries = append(entries, PrefEntry{
				Path:     filepath.Join(dir, e.Name()),
				File:     e.Name(),
				Location: loc,
				SizeKB:   size,
			})
		}
	}

	return entries
}

func scanURLSchemes(ctx context.Context, searchFilter string) []SchemeEntry {
	appSources := []string{"/Applications", "/System/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}
	var matched []SchemeEntry
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
						matched = append(matched, SchemeEntry{
							Scheme:  scheme,
							AppName: strings.TrimSuffix(filepath.Base(path), ".app"),
							AppPath: path,
						})
					}
				}
			}
			return filepath.SkipDir
		})
	}
	return matched
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

// ------- Generic action tools (replace per-item generated tools) -------

func genericLaunchAppTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_launch_app",
		Description: "Launch an application by name (e.g. 'Safari', 'Spotify'). Use search_apps first to find available apps.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Spotify', 'Notes') — required"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["name"].(string)
			if appName == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			out, err := runCmd("open", "-a", appName)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to launch %s: %s", appName, out)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Launched %s", appName)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericAppInfoTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_app_info",
		Description: "Get information about an installed application (path, bundle ID, version). Use search_apps first to discover apps.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Spotify') — required"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["name"].(string)
			if appName == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}

			// Find the app using mdfind
			out, err := runCmd("mdfind", fmt.Sprintf("kMDItemKind == 'Application' && kMDItemDisplayName == '%s'c", appName))
			if err != nil || out == "" {
				return common.Conversation{}, fmt.Errorf("application '%s' not found", appName)
			}
			paths := strings.Split(strings.TrimSpace(out), "\n")
			if len(paths) == 0 || paths[0] == "" {
				return common.Conversation{}, fmt.Errorf("application '%s' not found", appName)
			}
			appPath := paths[0]

			bundleId, _ := runCmd("mdls", "-name", "kMDItemCFBundleIdentifier", "-raw", appPath)
			version, _ := runCmd("mdls", "-name", "kMDItemVersion", "-raw", appPath)

			info := fmt.Sprintf("Name: %s\nPath: %s\nBundle ID: %s\nVersion: %s",
				appName, appPath, strings.TrimSpace(bundleId), strings.TrimSpace(version))
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: info}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericBrewUpgradeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_brew_upgrade",
		Description: "Upgrade a Homebrew formula or cask by package name. Use search_brew first to discover available packages.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"package": {Type: jsonschema.String, Description: "Homebrew package name (e.g. 'curl', 'node', 'firefox') — required"},
				"type":    {Type: jsonschema.String, Description: "Package type: 'formula', 'cask', or '' (auto-detect, default)"},
			},
			Required: []string{"package"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			pkgName, _ := args["package"].(string)
			if pkgName == "" {
				return common.Conversation{}, fmt.Errorf("package is required")
			}
			out, err := runCmd("brew", "upgrade", pkgName)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to upgrade %s: %s", pkgName, out)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericServiceStartTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_service_start",
		Description: "Start a launchd service/agent/daemon by plist label. Use search_services or search_agents first to discover available services.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Launchd service label (e.g. 'com.apple.ScreenSharing', 'nginx') — required"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			// Try user agent dirs first, then system
			dirs := []string{
				filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents"),
				"/Library/LaunchAgents",
				"/Library/LaunchDaemons",
				"/System/Library/LaunchDaemons",
			}
			for _, dir := range dirs {
				plistPath := filepath.Join(dir, name+".plist")
				if _, err := os.Stat(plistPath); err == nil {
					out, err := runCmd("launchctl", "load", plistPath)
					if err != nil {
						return common.Conversation{}, fmt.Errorf("failed to start %s: %s", name, out)
					}
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Started %s", name)}, nil
				}
			}
			return common.Conversation{}, fmt.Errorf("service '%s' not found in any launchd directory", name)
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericServiceStopTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_service_stop",
		Description: "Stop a launchd service/agent/daemon by plist label. Use search_services or search_agents first to discover available services.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Launchd service label (e.g. 'com.apple.ScreenSharing', 'nginx') — required"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			dirs := []string{
				filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents"),
				"/Library/LaunchAgents",
				"/Library/LaunchDaemons",
				"/System/Library/LaunchDaemons",
			}
			for _, dir := range dirs {
				plistPath := filepath.Join(dir, name+".plist")
				if _, err := os.Stat(plistPath); err == nil {
					out, err := runCmd("launchctl", "unload", plistPath)
					if err != nil {
						return common.Conversation{}, fmt.Errorf("failed to stop %s: %s", name, out)
					}
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Stopped %s", name)}, nil
				}
			}
			return common.Conversation{}, fmt.Errorf("service '%s' not found in any launchd directory", name)
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericServiceStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_service_status",
		Description: "Get the status of a launchd service/agent/daemon by plist label. Use search_services or search_agents first to discover available services.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Launchd service label (e.g. 'com.apple.ScreenSharing', 'nginx') — required"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			status := getLaunchdStatus(name)
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("%s status: %s", name, status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericPrefsReadTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_prefs_read",
		Description: "Read a macOS preference value by domain and key. Use search_prefs first to discover available preference domains.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"domain": {Type: jsonschema.String, Description: "Preference domain name, e.g. 'com.apple.finder' (required)"},
				"key":    {Type: jsonschema.String, Description: "Preference key to read (required)"},
			},
			Required: []string{"domain", "key"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			domain, _ := args["domain"].(string)
			key, _ := args["key"].(string)
			if domain == "" || key == "" {
				return common.Conversation{}, fmt.Errorf("domain and key are required")
			}
			out, err := runCmd("defaults", "read", domain, key)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read %s %s: %s", domain, key, out)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func genericPrefsWriteTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_prefs_write",
		Description: "Write a macOS preference value by domain, key, value, and type.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"domain": {Type: jsonschema.String, Description: "Preference domain name, e.g. 'com.apple.finder' (required)"},
				"key":    {Type: jsonschema.String, Description: "Preference key to write (required)"},
				"value":  {Type: jsonschema.String, Description: "Value to set (required)"},
				"type":   {Type: jsonschema.String, Description: "Value type: 'string' (default), 'int', 'bool', 'float'"},
			},
			Required: []string{"domain", "key", "value"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			domain, _ := args["domain"].(string)
			key, _ := args["key"].(string)
			value, _ := args["value"].(string)
			valType, _ := args["type"].(string)
			if domain == "" || key == "" {
				return common.Conversation{}, fmt.Errorf("domain, key, and value are required")
			}
			if valType == "" {
				valType = "string"
			}
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
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Set %s %s = %s", domain, key, value)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

// ------- Generic search tools -------

func searchAppsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_apps",
		Description: "Search installed applications. Returns name, path, bundle ID, and version for each matching app.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter apps by name"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 50)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			limit := 50
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			apps := scanInstalledApps(ctx, searchFilter, limit)
			if apps == nil {
				apps = []DiscoveredApp{}
			}
			result, _ := json.MarshalIndent(apps, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchBrewTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_brew",
		Description: "Search installed Homebrew formulae and casks. Returns name, version, type, and description.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter packages by name"},
				"type":   {Type: jsonschema.String, Description: "Package type: 'formula', 'cask', or 'all' (default)"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum results (default 50)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			pkgType := "all"
			if t, ok := args["type"].(string); ok {
				pkgType = t
			}
			limit := 50
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			pkgs := scanHomebrew(ctx, searchFilter, pkgType, limit)
			if pkgs == nil {
				pkgs = []DiscoveredPackage{}
			}
			result, _ := json.MarshalIndent(pkgs, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchPrefsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_prefs",
		Description: "Search preference plist files (domains). Returns path, file name, location (user/system), and size.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search":   {Type: jsonschema.String, Description: "Search term to filter by filename"},
				"location": {Type: jsonschema.String, Description: "Location: 'user', 'system', or 'all' (default)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			location := "all"
			if l, ok := args["location"].(string); ok {
				location = l
			}
			entries := scanPreferenceDomains(ctx, searchFilter, location)
			if entries == nil {
				entries = []PrefEntry{}
			}
			result, _ := json.MarshalIndent(entries, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchServicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_services",
		Description: "Search launchd services (system daemons). Returns name, status (running/stopped), and plist path.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			services := discoverServices("/Library/LaunchDaemons", searchFilter)
			if services == nil {
				services = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(services, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchAgentsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_agents",
		Description: "Search launch agents (user and system). Returns name, status (running/stopped), and plist path.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter by name"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			userDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
			combined := discoverServices(userDir, searchFilter)
			sysAgents := discoverServices("/Library/LaunchAgents", searchFilter)
			combined = append(combined, sysAgents...)
			if combined == nil {
				combined = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(combined, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchURLSchemesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_url_schemes",
		Description: "Search URL schemes registered by installed apps. Returns scheme, app name, and app path.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter URL schemes"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			entries := scanURLSchemes(ctx, searchFilter)
			if entries == nil {
				entries = []SchemeEntry{}
			}
			result, _ := json.MarshalIndent(entries, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func searchDaemonsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "search_daemons",
		Description: "Search system daemon plist files. Returns name, status, and plist path.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"search": {Type: jsonschema.String, Description: "Search term to filter daemons"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			searchFilter := ""
			if s, ok := args["search"].(string); ok {
				searchFilter = s
			}
			dirs := []string{"/Library/LaunchDaemons", "/System/Library/LaunchDaemons"}
			var daemons []DiscoveredService
			for _, dir := range dirs {
				daemons = append(daemons, discoverServices(dir, searchFilter)...)
			}
			if daemons == nil {
				daemons = []DiscoveredService{}
			}
			result, _ := json.MarshalIndent(daemons, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

func plistReadTool() common.MCPTool {
	return common.MCPTool{
		Name:        "plist_read",
		Description: "Read the contents of a plist file. Handles both XML and binary plists by using plutil to convert.",
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
				out, err = runCmd("/usr/libexec/PlistBuddy", "-c", fmt.Sprintf("Print :%s", key), path)
				if err != nil {
					domain := strings.TrimSuffix(filepath.Base(path), ".plist")
					out, err = runCmd("defaults", "read", domain, key)
				}
			} else {
				out, err = runCmd("plutil", "-convert", "xml1", "-o", "-", path)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to read plist %s: %s", path, err.Error())
				}
			}

			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read plist: %s", err.Error())
			}

			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
	}
}

// ------- Main entry point -------

func GetPerItemTools(ctx context.Context, api plugin.API) []common.MCPTool {
	var tools []common.MCPTool

	// Generic action tools (replacing thousands of per-item generated tools)
	tools = append(tools, genericLaunchAppTool())
	tools = append(tools, genericAppInfoTool())
	tools = append(tools, genericBrewUpgradeTool())
	tools = append(tools, genericServiceStartTool())
	tools = append(tools, genericServiceStopTool())
	tools = append(tools, genericServiceStatusTool())
	tools = append(tools, genericPrefsReadTool())
	tools = append(tools, genericPrefsWriteTool())

	// Search/discovery tools (scan on demand)
	tools = append(tools, searchAppsTool())
	tools = append(tools, searchBrewTool())
	tools = append(tools, searchPrefsTool())
	tools = append(tools, searchServicesTool())
	tools = append(tools, searchAgentsTool())
	tools = append(tools, searchURLSchemesTool())
	tools = append(tools, searchDaemonsTool())
	tools = append(tools, plistReadTool())

	api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Generated %d per-item tools (search + generic action)", len(tools)))
	return tools
}
