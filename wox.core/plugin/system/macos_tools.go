package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"wox/common"
	"wox/util/shell"

	"github.com/tmc/langchaingo/jsonschema"
)

func GetMacOSTools() []common.MCPTool {
	return []common.MCPTool{
		diskUsageTool(),
		memoryUsageTool(),
		cpuLoadTool(),
		batteryInfoTool(),
		uptimeTool(),
		macosVersionTool(),
		wifiSsidTool(),
		wifiSignalTool(),
		ipAddressTool(),
		activeConnectionsTool(),
		displayResolutionTool(),
		brightnessTool(),
		volumeControlTool(),
		audioOutputDevicesTool(),
		muteToggleTool(),
		microphoneStatusTool(),
		batteryPercentageTool(),
		powerSourceTool(),
		topProcessesTool(),
		launchdServicesTool(),
		volumeListTool(),
		diskFreeSpaceTool(),
		trashSizeTool(),
		emptyTrashTool(),
		lockScreenTool(),
		sleepDisplayTool(),
		screensaverTool(),
		darkModeToggleTool(),
		nightShiftToggleTool(),
		hideDesktopIconsTool(),
		systemLocaleTool(),
		keyboardLayoutTool(),
		timezoneTool(),
		gatekeeperStatusTool(),
		sipStatusTool(),
		filevaultStatusTool(),
		sshStatusTool(),
		remoteLoginTool(),
		airdropStatusTool(),
		bluetoothStatusTool(),
		bluetoothDevicesTool(),
		networkInterfacesTool(),
		dnsConfigTool(),
		proxySettingsTool(),
		firewallStatusTool(),
		sleepPreventionTool(),
		cacheSizeTool(),
		swapUsageTool(),
		thermalStateTool(),
		poweredOnTimeTool(),
	}
}

func diskUsageTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_disk_usage",
		Description: "Get disk usage information (total, used, available) for all mounted volumes",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("df", "-h")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func memoryUsageTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_memory_usage",
		Description: "Get memory usage information (total, used, wired, compressed)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("vm_stat")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func cpuLoadTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_cpu_load",
		Description: "Get current CPU load and usage breakdown",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("top", "-l", "1", "-n", "0", "-stats", "cpu")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func batteryInfoTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_battery_info",
		Description: "Get battery health information including cycle count, condition, and charge level",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pmset", "-g", "batt")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func uptimeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_uptime",
		Description: "Get system uptime information",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("uptime")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func macosVersionTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_version",
		Description: "Get macOS version and build information",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("sw_vers")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func wifiSsidTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_wifi_ssid",
		Description: "Get current connected Wi-Fi SSID",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func wifiSignalTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_wifi_signal",
		Description: "Get current Wi-Fi signal strength and quality",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func ipAddressTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ip_address",
		Description: "Get current IP address (local and external)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			local, _ := runCmd("ipconfig", "getifaddr", "en0")
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Local IP: %s", strings.TrimSpace(local))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func activeConnectionsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_active_connections",
		Description: "List active network connections",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("lsof", "-i", "-n", "-P")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func displayResolutionTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_display_resolution",
		Description: "Get display resolution and arrangement information",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("system_profiler", "SPDisplaysDataType")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func brightnessTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_brightness",
		Description: "Get current display brightness level",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("brightness", "-l")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func volumeControlTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_volume",
		Description: "Get current audio output volume level (0-100)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "output volume of (get volume settings)")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Volume: %s%%", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func audioOutputDevicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_audio_output_devices",
		Description: "List available audio output devices",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("system_profiler", "SPAudioDataType")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func muteToggleTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mute_status",
		Description: "Get current mute status of the system",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "output muted of (get volume settings)")
			if err != nil {
				return common.Conversation{}, err
			}
			muted := strings.TrimSpace(out) == "true"
			status := "unmuted"
			if muted {
				status = "muted"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Audio is %s", status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func microphoneStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_microphone_status",
		Description: "Get microphone input level or status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "input volume of (get volume settings)")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Microphone input volume: %s%%", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func batteryPercentageTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_battery_percentage",
		Description: "Get current battery percentage",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pmset", "-g", "batt")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func powerSourceTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_power_source",
		Description: "Get current power source (battery or AC adapter)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pmset", "-g", "batt")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func topProcessesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_top_processes",
		Description: "List top processes by CPU and memory usage",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"count": {Type: jsonschema.Integer, Description: "Number of processes to show (default 10)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("ps", "aux", "--sort=-%cpu")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func launchdServicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_launchd_services",
		Description: "List all running launchd services",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("launchctl", "list")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func volumeListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_volume_list",
		Description: "List all mounted volumes and their types",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("diskutil", "list")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func diskFreeSpaceTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_disk_free_space",
		Description: "Get available free disk space on the startup disk",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("df", "-h", "/")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func trashSizeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_trash_size",
		Description: "Get the size of the trash",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("du", "-sh", "~/.Trash/")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Trash is empty or cannot be accessed"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Trash size: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func emptyTrashTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_empty_trash",
		Description: "Empty the trash",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			err := exec.Command("osascript", "-e", "tell application \"Finder\" to empty trash").Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to empty trash: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Trash emptied successfully"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func lockScreenTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_lock_screen",
		Description: "Lock the screen immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			err := exec.Command("osascript", "-e", "tell application \"System Events\" to keystroke \"q\" using {command down, control down}").Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to lock screen: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Screen locked"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func sleepDisplayTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_sleep_display",
		Description: "Put the display to sleep immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			err := exec.Command("pmset", "sleepnow").Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to sleep display: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Display set to sleep"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func screensaverTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_start_screensaver",
		Description: "Start the screensaver immediately",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			err := exec.Command("open", "-a", "ScreenSaverEngine").Run()
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to start screensaver: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Screensaver started"}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func darkModeToggleTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_dark_mode_status",
		Description: "Get current dark mode status (on/off)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "tell application \"System Events\" to tell appearance preferences to get dark mode")
			if err != nil {
				return common.Conversation{}, err
			}
			isDark := strings.TrimSpace(out) == "true"
			status := "light mode"
			if isDark {
				status = "dark mode"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("System is in %s", status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func nightShiftToggleTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_night_shift_status",
		Description: "Get Night Shift status (on/off/scheduled)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "tell application \"System Events\" to tell display preferences to get Night Shift enabled")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Night Shift status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Night Shift: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func hideDesktopIconsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_desktop_icons_visibility",
		Description: "Check if desktop icons are visible or hidden",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("defaults", "read", "com.apple.finder", "CreateDesktop")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Desktop icons: visible (default)"}, nil
			}
			hidden := strings.TrimSpace(out) == "false"
			status := "visible"
			if hidden {
				status = "hidden"
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Desktop icons are %s", status)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func systemLocaleTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_system_locale",
		Description: "Get system locale and language settings",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("system_profiler", "SPSoftwareDataType")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func keyboardLayoutTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_keyboard_layout",
		Description: "Get current keyboard layout",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", "tell application \"System Events\" to tell current local to get its name")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Keyboard layout: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Keyboard layout: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func timezoneTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_timezone",
		Description: "Get current system timezone",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("systemsetup", "-gettimezone")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func gatekeeperStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_gatekeeper_status",
		Description: "Get Gatekeeper security status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("spctl", "--status")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Gatekeeper status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("Gatekeeper: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func sipStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_sip_status",
		Description: "Get System Integrity Protection (SIP) status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("csrutil", "status")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "SIP status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func filevaultStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_filevault_status",
		Description: "Get FileVault disk encryption status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("fdesetup", "status")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "FileVault status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func sshStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ssh_status",
		Description: "Check if SSH (Remote Login) is enabled",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("systemsetup", "-getremotelogin")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Remote Login: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func remoteLoginTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_remote_login",
		Description: "Check if Remote Login (SSH) is enabled",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("systemsetup", "-getremotelogin")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Remote Login: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func airdropStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_airdrop_status",
		Description: "Get AirDrop discoverability status",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("osascript", "-e", `tell application "System Events" to tell process "ControlCenter" to exists`)
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "AirDrop status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("AirDrop status: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func bluetoothStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_bluetooth_status",
		Description: "Get Bluetooth power status (on/off)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("system_profiler", "SPBluetoothDataType")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Bluetooth status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func bluetoothDevicesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_bluetooth_devices",
		Description: "List paired Bluetooth devices",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("system_profiler", "SPBluetoothDataType")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "No Bluetooth devices found"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func networkInterfacesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_network_interfaces",
		Description: "List all network interfaces and their configurations",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("ifconfig")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func dnsConfigTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_dns_config",
		Description: "Get DNS resolver configuration",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("scutil", "--dns")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func proxySettingsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_proxy_settings",
		Description: "Get network proxy settings",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("scutil", "--proxy")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func firewallStatusTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_firewall_status",
		Description: "Get firewall status (enabled/disabled)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Firewall status: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func sleepPreventionTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_sleep_prevention",
		Description: "Check if sleep is being prevented by any processes",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pmset", "-g", "assertions")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func cacheSizeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_cache_size",
		Description: "Get system cache size usage",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("du", "-sh", "~/Library/Caches")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Cache size: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: fmt.Sprintf("User cache size: %s", strings.TrimSpace(out))}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func swapUsageTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_swap_usage",
		Description: "Get swap file usage information",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("sysctl", "vm.swapusage")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func thermalStateTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_thermal_state",
		Description: "Get system thermal state (nominal, fair, serious, critical)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("pmset", "-g", "therm")
			if err != nil {
				return common.Conversation{Role: common.ConversationRoleAssistant, Text: "Thermal state: unknown"}, nil
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func poweredOnTimeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_powered_on_time",
		Description: "Get how long the system has been powered on (uptime)",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			out, err := runCmd("uptime")
			if err != nil {
				return common.Conversation{}, err
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: out}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// Run a shell command and return stdout as string
func runCmd(name string, args ...string) (string, error) {
	cmd := shell.BuildCommand(name, nil, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command %s failed: %s (stderr: %s)", name, err.Error(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("command %s failed: %s", name, err.Error())
	}
	// Return empty string for empty output
	if len(out) == 0 {
		return "", nil
	}
	// Filter out non-printable control chars except newlines and tabs
	var sb strings.Builder
	for _, r := range string(out) {
		if r == '\n' || r == '\t' || r >= 32 {
			sb.WriteRune(r)
		}
	}
	return sb.String(), nil
}

func init() {
	// Check for availability of key commands
	_, err := exec.LookPath("osascript")
	if err != nil {
		fmt.Fprintf(os.Stderr, "macOS tools: osascript not found, tools may not work correctly: %s\n", err.Error())
	}
}
