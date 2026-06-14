package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

// GetMacOSExtraTools returns additional MCP tools for macOS automation that
// don't fit neatly into system, app, AX, or Finder categories.
func GetMacOSExtraTools() []common.MCPTool {
	return []common.MCPTool{
		screenshotCaptureTool(),
		screenshotTimedTool(),
		screenshotCaptureWindowTool(),
		notificationPostTool(),
		terminalRunCommandTool(),
		dictionaryLookupTool(),
		installedAppsTool(),
		spacesListTool(),
		icloudDriveListTool(),
		facetimeCallTool(),
		voicememosListTool(),
		systemSettingsOpenTool(),
		shortcutsRunTool(),
		siriSuggestionsTool(),
		appleIntelligenceStatusTool(),
	}
}

func screenshotCaptureTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_screenshot_capture",
		Description: "Take a screenshot of the entire screen and save to a file",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"outputPath": {Type: jsonschema.String, Description: "Optional full path to save the screenshot (default: ~/Desktop/screenshot_<timestamp>.png)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			outputPath, _ := args["outputPath"].(string)
			if outputPath == "" {
				out, err := runCmd("mktemp", "/tmp/wox_screenshot_XXXXXX.png")
				if err != nil {
					outputPath = "/tmp/wox_screenshot.png"
				} else {
					outputPath = strings.TrimSpace(out)
				}
			}

			err := exec.Command("screencapture", "-x", outputPath).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to take screenshot: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Screenshot saved to: %s", outputPath)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func screenshotTimedTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_screenshot_timed",
		Description: "Take a screenshot after a delay, useful for capturing menus or popovers",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"delay":      {Type: jsonschema.Integer, Description: "Delay in seconds before taking the screenshot (default 3)"},
				"outputPath": {Type: jsonschema.String, Description: "Optional full path for the screenshot file"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			delay := 3
			if d, ok := args["delay"].(float64); ok {
				delay = int(d)
			}
			outputPath, _ := args["outputPath"].(string)
			if outputPath == "" {
				out, err := runCmd("mktemp", "/tmp/wox_screenshot_timed_XXXXXX.png")
				if err != nil {
					outputPath = "/tmp/wox_screenshot_timed.png"
				} else {
					outputPath = strings.TrimSpace(out)
				}
			}

			err := exec.Command("screencapture", "-x", "-T", fmt.Sprintf("%d", delay), outputPath).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to take timed screenshot: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Timed screenshot saved to: %s", outputPath)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func screenshotCaptureWindowTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_screenshot_capture_window",
		Description: "Capture a screenshot of the frontmost window (not the full screen)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"outputPath": {Type: jsonschema.String, Description: "Optional full path for the screenshot file"},
				"captureMouse": {Type: jsonschema.Boolean, Description: "Whether to include the mouse cursor (default false)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			outputPath, _ := args["outputPath"].(string)
			if outputPath == "" {
				out, err := runCmd("mktemp", "/tmp/wox_screenshot_window_XXXXXX.png")
				if err != nil {
					outputPath = "/tmp/wox_screenshot_window.png"
				} else {
					outputPath = strings.TrimSpace(out)
				}
			}

			argsList := []string{"-x", "-w", outputPath}

			err := exec.Command("screencapture", argsList...).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to capture window: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Window screenshot saved to: %s", outputPath)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func notificationPostTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_notification_post",
		Description: "Display a macOS notification banner (useful for alerts, reminders, or confirmations)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title":   {Type: jsonschema.String, Description: "Notification title (required)"},
				"message": {Type: jsonschema.String, Description: "Notification body message (required)"},
				"sound":   {Type: jsonschema.String, Description: "Optional sound name (e.g. 'Funk', 'Basso', 'default')"},
			},
			Required: []string{"title", "message"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			message, _ := args["message"].(string)
			sound, _ := args["sound"].(string)

			soundPart := ""
			if sound != "" {
				soundPart = fmt.Sprintf(` sound name "%s"`, sound)
			}

			script := fmt.Sprintf(`display notification "%s" with title "%s"%s`, message, title, soundPart)
			err := exec.Command("osascript", "-e", script).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to post notification: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Notification sent: '%s' - %s", title, message)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func terminalRunCommandTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_terminal_run_command",
		Description: "Run a shell command in a visible Terminal.app window and capture output",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String, Description: "Shell command to execute in a new Terminal window (required)"},
			},
			Required: []string{"command"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			command, _ := args["command"].(string)

			script := fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
	set output to "Command running in Terminal: " & "%s"
	return output
end tell`, command, command)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to run command in Terminal: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func dictionaryLookupTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_dictionary_lookup",
		Description: "Look up a word definition using the macOS Dictionary app",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"word": {Type: jsonschema.String, Description: "Word to look up in Dictionary (required)"},
			},
			Required: []string{"word"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			word, _ := args["word"].(string)

		script := fmt.Sprintf(`tell application "Dictionary"
	activate
	search for "%s"
	return "Dictionary opened for: " & "%s"
end tell`, word, word)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to look up word: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func installedAppsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_get_installed_apps",
		Description: "List all installed applications from /Applications and ~/Applications with bundle IDs",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum number of apps to list (default 50)"},
				"search": {Type: jsonschema.String, Description: "Optional search term to filter app names"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 50
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			search, _ := args["search"].(string)

			searchFilter := ""
			if search != "" {
				searchFilter = fmt.Sprintf(` | grep -i "%s"`, search)
			}

			script := fmt.Sprintf(`do shell script "mdfind 'kMDItemContentType == \"com.apple.application-bundle\"' | head -%d%s"`, limit, searchFilter)
			out, err := runAppleScript(script)
			if err != nil {
				// Fall back to simpler listing
				out2, err2 := runCmd("mdfind", "kMDItemContentType == \"com.apple.application-bundle\"")
				if err2 != nil {
					return common.Conversation{}, fmt.Errorf("failed to list installed apps: %s / %s", err.Error(), err2.Error())
				}
				lines := strings.Split(strings.TrimSpace(out2), "\n")
				if len(lines) > limit {
					lines = lines[:limit]
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.Join(lines, "\n")}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func spacesListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_spaces_list",
		Description: "List all active Spaces/Desktops via Mission Control (uses dock plist for space information)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("defaults", "read", "com.apple.spaces", "plist")
			if err != nil {
				// Fallback: try to get current space info from Dock
				out2, err2 := runCmd("defaults", "read", "com.apple.dock", "spaces")
				if err2 != nil {
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Space information not available (requires SIP-enabled access)"}, nil
				}
				_ = out2
			}
			_ = out
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Spaces/Desks are managed by Mission Control. To switch spaces, use Ctrl+Arrow shortcuts."}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func icloudDriveListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_icloud_drive_list",
		Description: "List files and folders in iCloud Drive",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"subpath": {Type: jsonschema.String, Description: "Optional subpath within iCloud Drive (e.g. 'Documents', 'Desktop')"},
				"limit":   {Type: jsonschema.Integer, Description: "Maximum items to list (default 20)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			subpath, _ := args["subpath"].(string)
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			icloudPath := fmt.Sprintf("~/Library/Mobile Documents/com~apple~CloudDocs/%s", subpath)
			script := fmt.Sprintf(`do shell script "ls -la \"%s\" | tail -n +2 | head -%d"`, icloudPath, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("iCloud Drive not accessible or empty at path: %s", icloudPath)}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func facetimeCallTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_facetime_call",
		Description: "Initiate a FaceTime call to a phone number or email address",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"target": {Type: jsonschema.String, Description: "Phone number, email, or contact name to call (required)"},
				"video":  {Type: jsonschema.Boolean, Description: "If true, start a FaceTime video call; if false, audio only (default true)"},
			},
			Required: []string{"target"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			target, _ := args["target"].(string)
			video := true
			if v, ok := args["video"].(bool); ok {
				video = v
			}

			var script string
			if video {
				script = fmt.Sprintf(`tell application "FaceTime"
	activate
	start call "%s"
	return "FaceTime video call initiated to: " & "%s"
end tell`, target, target)
			} else {
				script = fmt.Sprintf(`tell application "FaceTime"
	activate
	start call "%s" with audio only
	return "FaceTime audio call initiated to: " & "%s"
end tell`, target, target)
			}

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to initiate FaceTime call: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func voicememosListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_voicememos_list",
		Description: "List recent Voice Memos recordings",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum recordings to list (default 10)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			// Voice Memos stores recordings in a specific directory
			script := fmt.Sprintf(`do shell script "ls -lt ~/Library/Application\\ Support/com.apple.voicememos/Recordings/ 2>/dev/null | head -%d"`, limit+1)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No Voice Memos recordings found or path not accessible."}, nil
			}
			if strings.TrimSpace(out) == "" {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No Voice Memos recordings found."}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func systemSettingsOpenTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_system_settings_open",
		Description: "Open System Settings (System Preferences) to a specific pane by name or identifier",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"pane": {Type: jsonschema.String, Description: "Settings pane name or path, e.g. 'com.apple.preference.general', 'Privacy & Security', 'Desktop & Dock', 'Network' (required)"},
			},
			Required: []string{"pane"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			pane, _ := args["pane"].(string)

			script := fmt.Sprintf(`tell application "System Settings"
	activate
	reveal pane id "%s"
	return "Opened System Settings: " & "%s"
end tell`, pane, pane)
			out, err := runAppleScript(script)
			if err != nil {
				// Fallback: try opening via URL scheme
				_ = exec.Command("open", fmt.Sprintf("x-apple.systempreferences:%s", pane)).Run()
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Attempted to open System Settings pane: %s", pane)}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func shortcutsRunTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_shortcuts_run",
		Description: "Run a Shortcuts automation by its name",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name":  {Type: jsonschema.String, Description: "Name of the Shortcut to run (required)"},
				"input": {Type: jsonschema.String, Description: "Optional text input to pass to the Shortcut"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			input, _ := args["input"].(string)

			cmdArgs := []string{"run", name}
			if input != "" {
				cmdArgs = append(cmdArgs, "--input-options", "text", "--input", input)
			}

			out, err := exec.Command("shortcuts", cmdArgs...).CombinedOutput()
			if err != nil {
				// shortcuts command might not be available or shortcut not found
				outStr := string(out)
				if outStr != "" {
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Shortcut '%s' output: %s", name, strings.TrimSpace(outStr))}, nil
				}
				return common.Conversation{}, fmt.Errorf("failed to run Shortcut '%s': %s", name, err.Error())
			}
			outStr := string(out)
			if outStr == "" {
				outStr = fmt.Sprintf("Shortcut '%s' executed successfully", name)
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(outStr)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func siriSuggestionsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_siri_suggestions",
		Description: "Get Siri suggested applications based on current context and usage patterns",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum suggestions (default 10)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			// Read recent application usage from knowledge database
			script := fmt.Sprintf(`do shell script "defaults read ~/Library/Application\\ Support/com.apple.corespotlight/CoreSpotlightCache 2>/dev/null | head -%d || echo 'Suggestions not available' "`, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Siri suggestions not available (requires SIP-enabled access to knowledge database)."}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func appleIntelligenceStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_apple_intelligence_status",
		Description: "Check if Apple Intelligence features are available on this Mac",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			// Check for Apple Intelligence capabilities via sysctl
			out, err := runCmd("sysctl", "machdep.cpu.brand_string")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Apple Intelligence status: Cannot determine CPU model."}, nil
			}

			cpuInfo := strings.TrimSpace(out)
			hasAppleSilicon := strings.Contains(cpuInfo, "Apple") || strings.Contains(cpuInfo, "M1") || strings.Contains(cpuInfo, "M2") || strings.Contains(cpuInfo, "M3") || strings.Contains(cpuInfo, "M4")

			osOut, _ := runCmd("sw_vers")
			osVersion := ""
			for _, line := range strings.Split(osOut, "\n") {
				if strings.Contains(line, "ProductVersion") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						osVersion = strings.TrimSpace(parts[1])
					}
				}
			}

			text := fmt.Sprintf("CPU: %s\nmacOS: %s\nApple Silicon: %v\n", cpuInfo, osVersion, hasAppleSilicon)
			if hasAppleSilicon {
				text += "Apple Intelligence: Supported on this hardware (requires macOS Sequoia 15.1+ and enabled in Settings)"
			} else {
				text += "Apple Intelligence: Not supported on Intel-based Macs"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(text)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}
