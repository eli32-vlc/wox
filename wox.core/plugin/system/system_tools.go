package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"wox/common"
	"wox/util/shell"

	"github.com/tmc/langchaingo/jsonschema"
)

func GetSystemTools() []common.MCPTool {
	return []common.MCPTool{
		systemLaunchApp(),
		systemSearchFiles(),
		systemClipboardRead(),
		systemClipboardWrite(),
		systemCalculate(),
		systemRunShellCommand(),
		systemLockScreen(),
		systemShutdown(),
		systemRestart(),
		systemSleep(),
		systemVolumeGet(),
		systemVolumeSet(),
		systemVolumeMute(),
		systemDarkModeGet(),
		systemDarkModeToggle(),
		systemMediaPlayPause(),
		systemMediaNext(),
		systemMediaPrevious(),
		systemMediaCurrent(),
		systemCaptureScreenshot(),
		systemSearchEmoji(),
		systemGetSelectedText(),
		systemBrowseDirectory(),
		systemOpenUrl(),
		systemHideShowDesktop(),
		systemEjectDisks(),
		systemToggleHiddenFiles(),
		systemSleepDisplay(),
		systemEmptyTrash(),
		systemQuitApp(),
		systemForceQuitApp(),
		systemGetFrontmostApp(),
		systemListRunningApps(),
		systemSendNotification(),
		systemSystemInfo(),
	}
}

func systemLaunchApp() common.MCPTool {
	return common.MCPTool{
		Name:        "system_launch_app",
		Description: "Launch an application by name or full path",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Spotify') or full path"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			// Try open -a first (launch by bundle name)
			if err := exec.Command("open", "-a", name).Run(); err != nil {
				// Fallback to open by path
				if err := exec.Command("open", name).Run(); err != nil {
					return common.Conversation{}, fmt.Errorf("failed to launch %s: %s", name, err.Error())
				}
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Launched %s", name)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSearchFiles() common.MCPTool {
	return common.MCPTool{
		Name:        "system_search_files",
		Description: "Search for files by name using Spotlight index. Returns matching file paths.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {Type: jsonschema.String, Description: "Filename or search term"},
				"limit": {Type: jsonschema.Integer, Description: "Maximum results to return (default 20)"},
			},
			Required: []string{"query"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			query, _ := args["query"].(string)
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			out, err := runCmd("mdfind", "-count", strconv.Itoa(limit), "kMDItemDisplayName == '*"+query+"*'c")
			if err != nil {
				// Fallback to find
				out, err = runCmd("find", "/", "-maxdepth", "5", "-iname", "*"+query+"*", "-type", "f")
				if err != nil {
					return common.Conversation{}, fmt.Errorf("search failed: %s", err.Error())
				}
			}
			lines := strings.Split(strings.TrimSpace(out), "\n")
			if len(lines) > limit {
				lines = lines[:limit]
			}
			result := strings.Join(lines, "\n")
			if result == "" {
				result = "No files found"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: result}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemClipboardRead() common.MCPTool {
	return common.MCPTool{
		Name:        "system_clipboard_read",
		Description: "Read the current text content of the clipboard",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pbpaste")
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read clipboard: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemClipboardWrite() common.MCPTool {
	return common.MCPTool{
		Name:        "system_clipboard_write",
		Description: "Write text content to the clipboard",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"text": {Type: jsonschema.String, Description: "Text to copy to clipboard"},
			},
			Required: []string{"text"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			text, _ := args["text"].(string)
			if text == "" {
				return common.Conversation{}, fmt.Errorf("text is required")
			}
			cmd := exec.Command("pbcopy")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err != nil {
				return common.Conversation{}, fmt.Errorf("failed to write clipboard: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Clipboard updated"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemCalculate() common.MCPTool {
	return common.MCPTool{
		Name:        "system_calculate",
		Description: "Evaluate a mathematical expression",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"expression": {Type: jsonschema.String, Description: "Math expression to evaluate, e.g. '2 + 2', 'sqrt(144)', '3 * 4.5'"},
			},
			Required: []string{"expression"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			expr, _ := args["expression"].(string)
			if expr == "" {
				return common.Conversation{}, fmt.Errorf("expression is required")
			}
			out, err := runCmd("osascript", "-l", "JavaScript", "-e", fmt.Sprintf("Math.round((%s) * 1e10) / 1e10", expr))
			if err != nil {
				// Try bc as fallback
				cmd := exec.Command("bc", "-l")
				cmd.Stdin = strings.NewReader(expr)
				if out2, err2 := cmd.Output(); err2 == nil {
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(string(out2))}, nil
				}
				return common.Conversation{}, fmt.Errorf("failed to calculate: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemRunShellCommand() common.MCPTool {
	return common.MCPTool{
		Name:        "system_run_command",
		Description: "Execute a shell command and return its output. Use with caution for trusted commands only.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String, Description: "Shell command to execute, e.g. 'ls -la /Applications'"},
			},
			Required: []string{"command"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			cmdStr, _ := args["command"].(string)
			if cmdStr == "" {
				return common.Conversation{}, fmt.Errorf("command is required")
			}
			cmd := shell.BuildCommand("bash", nil, "-c", cmdStr)
			out, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					return common.Conversation{}, fmt.Errorf("command failed: %s (stderr: %s)", err.Error(), string(exitErr.Stderr))
				}
				return common.Conversation{}, fmt.Errorf("command failed: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemLockScreen() common.MCPTool {
	return common.MCPTool{
		Name:        "system_lock_screen",
		Description: "Lock the screen immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell application "System Events" to keystroke "q" using {command down, control down}`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Screen locked"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemShutdown() common.MCPTool {
	return common.MCPTool{
		Name:        "system_shutdown",
		Description: "Shut down the computer immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell app "System Events" to shut down`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Shutting down..."}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemRestart() common.MCPTool {
	return common.MCPTool{
		Name:        "system_restart",
		Description: "Restart the computer immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell app "System Events" to restart`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Restarting..."}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSleep() common.MCPTool {
	return common.MCPTool{
		Name:        "system_sleep",
		Description: "Put the computer to sleep immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("pmset", "sleepnow").Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Computer going to sleep"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemVolumeGet() common.MCPTool {
	return common.MCPTool{
		Name:        "system_volume_get",
		Description: "Get current audio output volume level (0-100) and mute status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "get volume settings")
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get volume: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemVolumeSet() common.MCPTool {
	return common.MCPTool{
		Name:        "system_volume_set",
		Description: "Set audio output volume to a specific level (0-100)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"level": {Type: jsonschema.Integer, Description: "Volume level from 0 (mute) to 100 (maximum)"},
			},
			Required: []string{"level"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			level, _ := args["level"].(float64)
			if err := exec.Command("osascript", "-e", fmt.Sprintf("set volume output volume %.0f", level)).Run(); err != nil {
				return common.Conversation{}, fmt.Errorf("failed to set volume: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Volume set to %.0f", level)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemVolumeMute() common.MCPTool {
	return common.MCPTool{
		Name:        "system_volume_mute",
		Description: "Toggle mute on/off, or set mute to a specific state",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"mute": {Type: jsonschema.Boolean, Description: "true to mute, false to unmute. Omit to toggle."},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if _, ok := args["mute"]; ok {
				mute, _ := args["mute"].(bool)
				state := "false"
				if mute {
					state = "true"
				}
				exec.Command("osascript", "-e", fmt.Sprintf("set volume output muted %s", state)).Run()
				status := "unmuted"
				if mute {
					status = "muted"
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Audio %s", status)}, nil
			}
			// Toggle
			out, _ := runCmd("osascript", "-e", "output muted of (get volume settings)")
			isMuted := strings.TrimSpace(out) == "true"
			newState := "false"
			status := "unmuted"
			if isMuted {
				newState = "true"
				status = "muted"
			}
			exec.Command("osascript", "-e", fmt.Sprintf("set volume output muted %s", newState)).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Audio %s", status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemDarkModeGet() common.MCPTool {
	return common.MCPTool{
		Name:        "system_dark_mode_get",
		Description: "Get the current dark/light mode state of macOS",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", `tell application "System Events" to tell appearance preferences to get dark mode`)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Dark mode status: unknown"}, nil
			}
			status := "light"
			if strings.TrimSpace(out) == "true" {
				status = "dark"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("System appearance: %s mode", status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemDarkModeToggle() common.MCPTool {
	return common.MCPTool{
		Name:        "system_dark_mode_toggle",
		Description: "Toggle macOS dark mode on or off. If mode is not specified, toggles between dark and light.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"enable": {Type: jsonschema.Boolean, Description: "true for dark mode, false for light mode. Omit to toggle."},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if v, ok := args["enable"]; ok {
				enable, _ := v.(bool)
				state := "false"
				label := "light"
				if enable {
					state = "true"
					label = "dark"
				}
				exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to tell appearance preferences to set dark mode to %s`, state)).Run()
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Switched to %s mode", label)}, nil
			}
			// Toggle
			out, _ := runCmd("osascript", "-e", `tell application "System Events" to tell appearance preferences to get dark mode`)
			isDark := strings.TrimSpace(out) == "true"
			newState := "false"
			label := "dark"
			if isDark {
				newState = "true"
				label = "light"
			}
			exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to tell appearance preferences to set dark mode to %s`, newState)).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Switched to %s mode", label)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemMediaPlayPause() common.MCPTool {
	return common.MCPTool{
		Name:        "system_media_play_pause",
		Description: "Toggle play/pause for the current media player",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell application "System Events" to key code 16`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Play/pause toggled"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemMediaNext() common.MCPTool {
	return common.MCPTool{
		Name:        "system_media_next",
		Description: "Skip to the next track in the current media player",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell application "System Events" to key code 17`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Next track"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemMediaPrevious() common.MCPTool {
	return common.MCPTool{
		Name:        "system_media_previous",
		Description: "Go to the previous track in the current media player",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("osascript", "-e", `tell application "System Events" to key code 18`).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Previous track"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemMediaCurrent() common.MCPTool {
	return common.MCPTool{
		Name:        "system_media_current",
		Description: "Get information about the currently playing media track (title, artist, album, app)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "System Events"
				set appList to {"Music", "Spotify"}
				repeat with appName in appList
					if application appName is running then
						tell application appName
							if player state is playing then
								return appName & ": " & name of current track & " - " & artist of current track
							end if
						end tell
					end if
				end repeat
				return "No media playing"
			end tell`
			out, err := runCmd("osascript", "-e", script)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No media playing"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemCaptureScreenshot() common.MCPTool {
	return common.MCPTool{
		Name:        "system_capture_screenshot",
		Description: "Capture a screenshot to the desktop. Supports full screen, selection, or window capture.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"type": {Type: jsonschema.String, Description: "Type of screenshot: 'full' (full screen), 'selection' (interactive selection), 'window' (active window). Default: 'full'"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			sType := "full"
			if t, ok := args["type"].(string); ok {
				sType = t
			}
			desktop := filepath.Join(os.Getenv("HOME"), "Desktop")
			timestamp := fmt.Sprintf("screenshot_%d", time.Now().UnixMilli())
			path := filepath.Join(desktop, timestamp+".png")

			switch sType {
			case "selection":
				exec.Command("screencapture", "-i", path).Run()
			case "window":
				exec.Command("screencapture", "-w", path).Run()
			default:
				exec.Command("screencapture", path).Run()
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Screenshot saved to %s", path)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSearchEmoji() common.MCPTool {
	return common.MCPTool{
		Name:        "system_search_emoji",
		Description: "Search for emojis by keyword or description. Returns matching emoji characters and their names.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {Type: jsonschema.String, Description: "Keyword to search for, e.g. 'smile', 'heart', 'fire', 'check'"}},
			Required: []string{"query"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return common.Conversation{}, fmt.Errorf("query is required")
			}
			// Use a basic built-in emoji map for common searches
			emojiMap := map[string]string{
				"smile": "😊", "grin": "😁", "laugh": "😂", "happy": "😊", "joy": "😂",
				"sad": "😢", "cry": "😢", "frown": "😞", "angry": "😠", "mad": "😡",
				"heart": "❤️", "love": "❤️", "fire": "🔥", "cool": "😎", "wink": "😉",
				"star": "⭐", "globe": "🌍", "moon": "🌙", "sun": "☀️", "cloud": "☁️",
				"rain": "🌧️", "snow": "❄️", "bolt": "⚡", "zap": "⚡", "thunder": "⚡",
				"check": "✅", "cross": "❌", "xmark": "❌", "warning": "⚠️", "info": "ℹ️",
				"question": "❓", "exclamation": "❗", "mail": "📧", "email": "📧",
				"phone": "📞", "call": "📞", "computer": "💻", "laptop": "💻", "keyboard": "⌨️",
				"mouse": "🖱️", "folder": "📁", "file": "📄", "lock": "🔒", "key": "🔑",
				"search": "🔍", "magnify": "🔍", "gear": "⚙️", "setting": "⚙️",
				"home": "🏠", "house": "🏠", "music": "🎵", "note": "🎵",
				"play": "▶️", "pause": "⏸️", "stop": "⏹️", "record": "⏺️",
				"up": "⬆️", "down": "⬇️", "left": "⬅️", "right": "➡️",
				"ok": "🆗", "new": "🆕", "free": "🆓", "top": "🔝",
				"bookmark": "🔖", "tag": "🏷️", "trash": "🗑️", "delete": "🗑️",
				"pencil": "✏️", "edit": "✏️", "write": "✏️", "pen": "🖊️",
				"calendar": "📅", "clock": "🕐", "time": "🕐", "alarm": "⏰",
				"bell": "🔔", "notification": "🔔", "speaker": "🔊", "sound": "🔊",
				"mute": "🔇", "camera": "📷", "photo": "📷", "image": "🖼️",
				"video": "🎬", "film": "🎬", "movie": "🎬",
				"bug": "🐛", "wrench": "🔧", "tool": "🔧", "hammer": "🔨",
				"light": "💡", "bulb": "💡", "idea": "💡",
			}
			queryLower := strings.ToLower(query)
			var results []string
			for key, emoji := range emojiMap {
				if strings.Contains(key, queryLower) || strings.Contains(queryLower, key) {
					results = append(results, fmt.Sprintf("%s %s", emoji, key))
				}
			}
			if len(results) == 0 {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("No emojis found for '%s'", query)}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.Join(results, "\n")}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemGetSelectedText() common.MCPTool {
	return common.MCPTool{
		Name:        "system_get_selected_text",
		Description: "Get the currently selected text from the frontmost application",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", `tell application "System Events" to get selection`)
			if err != nil {
				// Try alternative: copy selection then read clipboard
				exec.Command("osascript", "-e", `tell application "System Events" to keystroke "c" using command down`).Run()
				clipOut, clipErr := runCmd("pbpaste")
				if clipErr != nil || strings.TrimSpace(clipOut) == "" {
					return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No text selected"}, nil
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(clipOut)}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemBrowseDirectory() common.MCPTool {
	return common.MCPTool{
		Name:        "system_browse_directory",
		Description: "List files and directories at a given path",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":  {Type: jsonschema.String, Description: "Directory path to browse. Defaults to home directory."},
				"depth": {Type: jsonschema.Integer, Description: "How many levels deep to list (default 1, max 3)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			dir := os.Getenv("HOME")
			if p, ok := args["path"].(string); ok && p != "" {
				dir = p
			}
			depth := 1
			if d, ok := args["depth"].(float64); ok {
				depth = int(d)
				if depth > 3 {
					depth = 3
				}
			}
			var result []string
			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				if rel == "." {
					return nil
				}
				levels := len(strings.Split(rel, string(os.PathSeparator)))
				if levels > depth {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				prefix := "📄"
				if d.IsDir() {
					prefix = "📁"
				}
				result = append(result, fmt.Sprintf("%s %s", prefix, rel))
				return nil
			})
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to browse %s: %s", dir, err.Error())
			}
			if len(result) == 0 {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Directory '%s' is empty", dir)}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.Join(result, "\n")}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemOpenUrl() common.MCPTool {
	return common.MCPTool{
		Name:        "system_open_url",
		Description: "Open a URL in the default browser",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"url": {Type: jsonschema.String, Description: "URL to open, e.g. 'https://example.com'"},
			},
			Required: []string{"url"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			url, _ := args["url"].(string)
			if url == "" {
				return common.Conversation{}, fmt.Errorf("url is required")
			}
			if err := exec.Command("open", url).Run(); err != nil {
				return common.Conversation{}, fmt.Errorf("failed to open URL: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Opened %s", url)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemHideShowDesktop() common.MCPTool {
	return common.MCPTool{
		Name:        "system_hide_show_desktop",
		Description: "Show or hide all windows to reveal the desktop. Toggle between show and hide.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"show": {Type: jsonschema.Boolean, Description: "true to show desktop, false to hide. Omit to toggle."},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			mode := "toggle"
			if v, ok := args["show"]; ok {
				if v.(bool) {
					mode = "reveal"
				} else {
					mode = "hide"
				}
			}
			if mode == "reveal" {
				exec.Command("osascript", "-e", `tell application "System Events" to key code 103 using {command down}`).Run()
			} else if mode == "hide" {
				// Just click on a window to restore - simulate by focusing front app
				exec.Command("osascript", "-e", `tell application "System Events" to key code 53`).Run()
			} else {
				exec.Command("osascript", "-e", `tell application "System Events" to key code 103 using {command down}`).Run()
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Desktop toggled"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemEjectDisks() common.MCPTool {
	return common.MCPTool{
		Name:        "system_eject_disks",
		Description: "Eject all removable disks or a specific disk by name",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Optional disk name to eject. If omitted, ejects all removable disks."},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			if name, ok := args["name"].(string); ok && name != "" {
				if err := exec.Command("diskutil", "eject", name).Run(); err != nil {
					return common.Conversation{}, fmt.Errorf("failed to eject %s: %s", name, err.Error())
				}
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Ejected %s", name)}, nil
			}
			out, err := runCmd("osascript", "-e", `tell application "Finder" to eject every disk whose ejectable is true`)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No ejectable disks found"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Ejected disks: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemToggleHiddenFiles() common.MCPTool {
	return common.MCPTool{
		Name:        "system_toggle_hidden_files",
		Description: "Toggle visibility of hidden files in Finder. When enabled, dotfiles are shown.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"show": {Type: jsonschema.Boolean, Description: "true to show hidden files, false to hide. Omit to toggle."},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			var show bool
			if v, ok := args["show"]; ok {
				show = v.(bool)
			} else {
				// Toggle: read current state
				out, _ := runCmd("defaults", "read", "com.apple.finder", "AppleShowAllFiles")
				show = strings.TrimSpace(out) != "NO" && strings.TrimSpace(out) != "0" && strings.TrimSpace(out) != "false"
				show = !show
			}
			val := "FALSE"
			label := "hidden"
			if show {
				val = "TRUE"
				label = "visible"
			}
			exec.Command("defaults", "write", "com.apple.finder", "AppleShowAllFiles", "-bool", val).Run()
			exec.Command("killall", "Finder").Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Hidden files are now %s", label)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSleepDisplay() common.MCPTool {
	return common.MCPTool{
		Name:        "system_sleep_display",
		Description: "Put the display to sleep immediately (computer stays on)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			exec.Command("pmset", "displaysleepnow").Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Display sleep requested"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemEmptyTrash() common.MCPTool {
	return common.MCPTool{
		Name:        "system_empty_trash",
		Description: "Empty the Finder trash",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			err := exec.Command("osascript", "-e", `tell application "Finder" to empty trash`).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to empty trash: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Trash emptied"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemQuitApp() common.MCPTool {
	return common.MCPTool{
		Name:        "system_quit_app",
		Description: "Quit an application gracefully by name",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Application name to quit, e.g. 'Safari', 'Spotify'"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			err := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "%s" to quit`, name)).Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to quit %s: %s", name, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Quit %s", name)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemForceQuitApp() common.MCPTool {
	return common.MCPTool{
		Name:        "system_force_quit_app",
		Description: "Force quit an application by name (immediate termination)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Application name to force quit, e.g. 'Safari'"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return common.Conversation{}, fmt.Errorf("name is required")
			}
			err := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "%s" to quit saving no`, name)).Run()
			if err != nil {
				// Try via kill
				if pidStr, pidErr := runCmd("pgrep", "-x", name); pidErr == nil {
					pidStr = strings.TrimSpace(pidStr)
					if pid, parseErr := strconv.Atoi(pidStr); parseErr == nil {
						exec.Command("kill", "-9", fmt.Sprintf("%d", pid)).Run()
					}
				}
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Force quit %s", name)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemGetFrontmostApp() common.MCPTool {
	return common.MCPTool{
		Name:        "system_get_frontmost_app",
		Description: "Get the name of the currently active (frontmost) application",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", `tell application "System Events" to get name of first application process whose frontmost is true`)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemListRunningApps() common.MCPTool {
	return common.MCPTool{
		Name:        "system_list_running_apps",
		Description: "List all currently running applications",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"include_hidden": {Type: jsonschema.Boolean, Description: "Include background/hidden apps (default false)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			includeHidden := false
			if v, ok := args["include_hidden"].(bool); ok {
				includeHidden = v
			}
			script := `tell application "System Events"
				set appList to {}
				set processes to every application process`
			if !includeHidden {
				script += ` whose visible is true`
			}
			script += `
				repeat with p in processes
					set end of appList to name of p
				end repeat
				return appList
			end tell`
			out, err := runCmd("osascript", "-e", script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list apps: %s", err.Error())
			}
			apps := strings.Split(strings.TrimSpace(out), ", ")
			result := strings.Join(apps, "\n")
			if result == "" {
				result = "No running applications found"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: result}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSendNotification() common.MCPTool {
	return common.MCPTool{
		Name:        "system_send_notification",
		Description: "Send a macOS notification with a title and message",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title":   {Type: jsonschema.String, Description: "Notification title"},
				"message": {Type: jsonschema.String, Description: "Notification body text"},
			},
			Required: []string{"title", "message"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			message, _ := args["message"].(string)
			script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
			exec.Command("osascript", "-e", script).Run()
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Notification sent"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}

func systemSystemInfo() common.MCPTool {
	return common.MCPTool{
		Name:        "system_get_info",
		Description: "Get a comprehensive summary of system information including hardware, OS, and resource usage",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			type SysInfo struct {
				Hostname     string `json:"hostname"`
				OSVersion    string `json:"osVersion"`
				Kernel       string `json:"kernel"`
				CPU          string `json:"cpu"`
				Memory       string `json:"memory"`
				Disk         string `json:"disk"`
				Uptime       string `json:"uptime"`
				Battery      string `json:"battery"`
				User         string `json:"user"`
				Architecture string `json:"architecture"`
			}

			info := SysInfo{}
			hostname, _ := runCmd("scutil", "--get", "ComputerName")
			info.Hostname = strings.TrimSpace(hostname)

			osVer, _ := runCmd("sw_vers", "-productVersion")
			info.OSVersion = strings.TrimSpace(osVer)

			kernel, _ := runCmd("uname", "-r")
			info.Kernel = strings.TrimSpace(kernel)

			arch, _ := runCmd("uname", "-m")
			info.Architecture = strings.TrimSpace(arch)

			cpu, _ := runCmd("sysctl", "-n", "machdep.cpu.brand_string")
			info.CPU = strings.TrimSpace(cpu)

			memTotal, _ := runCmd("sysctl", "-n", "hw.memsize")
			if memGB, err := strconv.ParseInt(strings.TrimSpace(memTotal), 10, 64); err == nil {
				info.Memory = fmt.Sprintf("%d GB", memGB/1024/1024/1024)
			}

			disk, _ := runCmd("df", "-h", "/")
			lines := strings.Split(disk, "\n")
			if len(lines) > 1 {
				info.Disk = strings.Join(lines[:2], "\n")
			}

			uptime, _ := runCmd("uptime")
			info.Uptime = strings.TrimSpace(uptime)

			batt, _ := runCmd("pmset", "-g", "batt")
			info.Battery = strings.TrimSpace(batt)

			user, _ := runCmd("whoami")
			info.User = strings.TrimSpace(user)

			result, _ := json.MarshalIndent(info, "", "  ")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: string(result)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "wox_system"},
	}
}
