package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

const cacheFileName = "ai_discovery_cache.json"
const cacheMaxAge = 30 * time.Minute

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

type AIDiscoveryCacheData struct {
	Apps        []DiscoveredApp     `json:"apps"`
	Brew        []DiscoveredPackage `json:"brew"`
	Services    []DiscoveredService `json:"services"`
	Agents      []DiscoveredService `json:"agents"`
	Daemons     []DiscoveredService `json:"daemons"`
	Preferences []PrefEntry         `json:"preferences"`
	Schemes     []SchemeEntry       `json:"schemes"`
	LastScan    int64               `json:"lastScan"`
}

var (
	cacheMu    sync.RWMutex
	cacheData  *AIDiscoveryCacheData
	cacheDirty bool
)

func aiCachePath() string {
	return filepath.Join(util.GetLocation().GetCacheDirectory(), cacheFileName)
}

func loadDiscoveryCache() *AIDiscoveryCacheData {
	cacheMu.RLock()
	if cacheData != nil && !cacheDirty {
		cacheMu.RUnlock()
		return cacheData
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cacheData != nil && !cacheDirty {
		return cacheData
	}

	data, err := os.ReadFile(aiCachePath())
	if err != nil {
		cacheData = &AIDiscoveryCacheData{}
		return cacheData
	}

	var cd AIDiscoveryCacheData
	if err := json.Unmarshal(data, &cd); err != nil {
		cacheData = &AIDiscoveryCacheData{}
		return cacheData
	}

	now := time.Now().UnixMilli()
	if now-cd.LastScan > cacheMaxAge.Milliseconds() {
		cacheData = &AIDiscoveryCacheData{}
		return cacheData
	}

	cacheData = &cd
	return cacheData
}

func saveDiscoveryCache(cd *AIDiscoveryCacheData) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	cd.LastScan = time.Now().UnixMilli()
	cacheData = cd
	cacheDirty = false

	dir := filepath.Dir(aiCachePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	data, err := json.MarshalIndent(cd, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(aiCachePath(), data, 0644)
}

func refreshPerItemTools(ctx context.Context) {
	cacheMu.Lock()
	cacheDirty = true
	cacheMu.Unlock()
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

// ------- Per-item tool generation -------

func generateAppTools(apps []DiscoveredApp) []common.MCPTool {
	var tools []common.MCPTool
	for _, app := range apps {
		appName := app.Name
		appPath := app.Path
		bundleId := app.BundleId

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("launch_%s", appName)),
			Description: fmt.Sprintf("Launch %s application", appName),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("open", "-a", appName)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to launch %s: %s", appName, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Launched %s", appName)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("info_%s", appName)),
			Description: fmt.Sprintf("Get information about %s application", appName),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				info := fmt.Sprintf("Name: %s\nPath: %s\nBundle ID: %s", appName, appPath, bundleId)
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: info}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})

		// URL schemes for this app
		plistPath := filepath.Join(appPath, "Contents", "Info.plist")
		if out, err := runCmd("plutil", "-convert", "xml1", "-o", "-", plistPath); err == nil {
			schemes := extractURLSchemesFromPlistXML(out)
			for _, scheme := range schemes {
				s := scheme
				tools = append(tools, common.MCPTool{
					Name:        safeToolName(fmt.Sprintf("open_%s", s)),
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
					ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
				})
			}
		}
	}
	return tools
}

func generateBrewTools(pkgs []DiscoveredPackage) []common.MCPTool {
	var tools []common.MCPTool
	for _, pkg := range pkgs {
		pkgName := pkg.Name
		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("brew_upgrade_%s", pkgName)),
			Description: fmt.Sprintf("Upgrade Homebrew %s %s", pkg.Type, pkgName),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("brew", "upgrade", pkgName)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to upgrade %s: %s", pkgName, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})
	}
	return tools
}

func generateServiceTools(services []DiscoveredService, prefix string) []common.MCPTool {
	var tools []common.MCPTool
	for _, svc := range services {
		name := svc.Name
		plistPath := svc.PlistPath

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("%s_start_%s", prefix, name)),
			Description: fmt.Sprintf("Start the %s %s", prefix, name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("launchctl", "load", plistPath)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to start %s: %s", name, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Started %s", name)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("%s_stop_%s", prefix, name)),
			Description: fmt.Sprintf("Stop the %s %s", prefix, name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				out, err := runCmd("launchctl", "unload", plistPath)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to stop %s: %s", name, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Stopped %s", name)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("%s_status_%s", prefix, name)),
			Description: fmt.Sprintf("Get the status of the %s %s", prefix, name),
			Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				status := getLaunchdStatus(name)
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("%s status: %s", name, status)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})
	}
	return tools
}

func generatePrefTools(entries []PrefEntry) []common.MCPTool {
	var tools []common.MCPTool
	seen := make(map[string]bool)
	for _, entry := range entries {
		domain := strings.TrimSuffix(entry.File, ".plist")
		if seen[domain] {
			continue
		}
		seen[domain] = true
		d := domain

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("prefs_read_%s", d)),
			Description: fmt.Sprintf("Read a preference key from %s", d),
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"key": {Type: jsonschema.String, Description: "Preference key to read"},
				},
			},
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				key, _ := args["key"].(string)
				out, err := runCmd("defaults", "read", d, key)
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to read %s %s: %s", d, key, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})

		tools = append(tools, common.MCPTool{
			Name:        safeToolName(fmt.Sprintf("prefs_write_%s", d)),
			Description: fmt.Sprintf("Write a preference value to %s", d),
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
					out, err = runCmd("defaults", "write", d, key, "-int", value)
				case "bool":
					out, err = runCmd("defaults", "write", d, key, "-bool", value)
				case "float":
					out, err = runCmd("defaults", "write", d, key, "-float", value)
				default:
					out, err = runCmd("defaults", "write", d, key, value)
				}
				if err != nil {
					return common.Conversation{}, fmt.Errorf("failed to write %s %s: %s", d, key, out)
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Set %s %s = %s (%s)", d, key, value, valType)}, nil
			},
			ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_per_item"},
		})
	}
	return tools
}

func safeToolName(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	result = strings.TrimLeft(result, "_-")
	if result == "" {
		return "tool"
	}
	return result
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
	cd := loadDiscoveryCache()

	// If cache is empty (stale or first run), do a full scan synchronously
	needsScan := false
	if cd.LastScan == 0 {
		needsScan = true
	} else {
		now := time.Now().UnixMilli()
		if now-cd.LastScan > cacheMaxAge.Milliseconds() {
			needsScan = true
		}
	}

	if needsScan {
		api.Log(ctx, plugin.LogLevelInfo, "AI: Scanning macOS for per-item tools (may take a few seconds)...")

		cd.Apps = scanInstalledApps(ctx, "", 300)
		cd.Brew = scanHomebrew(ctx, "", "all", 200)
		cd.Services = discoverServices("/Library/LaunchDaemons", "")
		cd.Agents = discoverServices(filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents"), "")
		agentSys := discoverServices("/Library/LaunchAgents", "")
		cd.Agents = append(cd.Agents, agentSys...)
		cd.Daemons = discoverServices("/Library/LaunchDaemons", "")
		daemonSys := discoverServices("/System/Library/LaunchDaemons", "")
		cd.Daemons = append(cd.Daemons, daemonSys...)
		cd.Preferences = scanPreferenceDomains(ctx, "", "all")
		cd.Schemes = scanURLSchemes(ctx, "")

		saveDiscoveryCache(cd)
		api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Scanned %d apps, %d brew, %d services, %d agents, %d daemons, %d prefs, %d schemes",
			len(cd.Apps), len(cd.Brew), len(cd.Services), len(cd.Agents), len(cd.Daemons), len(cd.Preferences), len(cd.Schemes)))
	}

	var tools []common.MCPTool

	tools = append(tools, generateAppTools(cd.Apps)...)
	tools = append(tools, generateBrewTools(cd.Brew)...)
	tools = append(tools, generateServiceTools(cd.Services, "service")...)
	tools = append(tools, generateServiceTools(cd.Agents, "agent")...)
	tools = append(tools, generateServiceTools(cd.Daemons, "daemon")...)
	tools = append(tools, generatePrefTools(cd.Preferences)...)

	// Generic search tools
	tools = append(tools, searchAppsTool())
	tools = append(tools, searchBrewTool())
	tools = append(tools, searchPrefsTool())
	tools = append(tools, searchServicesTool())
	tools = append(tools, searchAgentsTool())
	tools = append(tools, searchURLSchemesTool())
	tools = append(tools, searchDaemonsTool())
	tools = append(tools, plistReadTool())

	api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Generated %d per-item tools", len(tools)))
	return tools
}
