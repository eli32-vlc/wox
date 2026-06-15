package system

import (
	"context"
	"fmt"
	"strings"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

// GetMacOSFinderTools returns MCP tools for Finder automation.
func GetMacOSFinderTools() []common.MCPTool {
	return []common.MCPTool{
		finderListWindowsTool(),
		finderGetSelectionTool(),
		finderSelectFileTool(),
		finderNewFolderTool(),
		finderDuplicateTool(),
		finderCompressTool(),
		finderGetInfoTool(),
		finderTagTool(),
	}
}

func finderListWindowsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_list_windows",
		Description: "List all open Finder windows with their file paths and positions",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Finder"
	set output to ""
	set windowList to (every window)
	if (count of windowList) is 0 then
		set output to "No Finder windows open"
	else
		repeat with w in windowList
			try
				set targetPath to (POSIX path of (target of w as alias))
				set {wx, wy} to position of w
				set {ww, wh} to dimensions of w
				set output to output & "Window: " & name of w & return
				set output to output & "  Path: " & targetPath & return
				set output to output & "  Position: (" & wx & ", " & wy & ") Size: " & ww & "x" & wh & return
				set output to output & "---" & return
			end try
		end repeat
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list Finder windows: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderGetSelectionTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_get_selection",
		Description: "Get the list of currently selected files/folders in the active Finder window",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Finder"
	set output to ""
	set selectionList to (selection)
	if (count of selectionList) is 0 then
		set output to "No files selected in Finder"
	else
		repeat with s in selectionList
			try
				set filePath to (POSIX path of (s as alias))
				set infoStr to ""
				if kind of s is not "" then
					set infoStr to kind of s
				end if
				set output to output & filePath
				if infoStr is not "" then
					set output to output & " (" & infoStr & ")"
				end if
				set output to output & return
			end try
		end repeat
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get Finder selection: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderSelectFileTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_select_file",
		Description: "Reveal and select a specific file or folder in a new Finder window",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Full file system path to reveal in Finder (required)"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)

			script := fmt.Sprintf(`tell application "Finder"
	try
		set targetFile to (POSIX file "%s") as alias
		reveal targetFile
		activate
		return "Revealed: " & "%s"
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path), escapeForAppleScript(path))
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to select file in Finder: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderNewFolderTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_new_folder",
		Description: "Create a new folder in the frontmost Finder window location or at a specified path",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"folderName": {Type: jsonschema.String, Description: "Name for the new folder (default: 'untitled folder')"},
				"parentPath": {Type: jsonschema.String, Description: "Optional parent directory path. If omitted, uses the frontmost Finder window location"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			folderName, _ := args["folderName"].(string)
			if folderName == "" {
				folderName = "untitled folder"
			}
			parentPath, _ := args["parentPath"].(string)

			var script string
			if parentPath != "" {
				script = fmt.Sprintf(`tell application "Finder"
	try
		set parentFolder to (POSIX file "%s") as alias
		set newFolder to make new folder at parentFolder with properties {name:"%s"}
		select newFolder
		return "Created folder: " & (POSIX path of (newFolder as alias))
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(parentPath), escapeForAppleScript(folderName))
			} else {
				script = fmt.Sprintf(`tell application "Finder"
	try
		set frontWindow to window 1
		set newFolder to make new folder at (target of frontWindow as alias) with properties {name:"%s"}
		select newFolder
		return "Created folder: " & (POSIX path of (newFolder as alias))
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(folderName))
			}

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to create folder: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderDuplicateTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_duplicate",
		Description: "Duplicate a file or folder in Finder",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Full path to the file/folder to duplicate (required)"},
				"name": {Type: jsonschema.String, Description: "Optional new name for the duplicate (default: original name + ' copy')"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)
			newName, _ := args["name"].(string)

			var script string
			if newName != "" {
				script = fmt.Sprintf(`tell application "Finder"
	try
		set sourceFile to (POSIX file "%s") as alias
		set parentFolder to (container of sourceFile) as alias
		set newFile to duplicate sourceFile to parentFolder
		set name of newFile to "%s"
		return "Duplicated to: " & (POSIX path of (newFile as alias))
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path), escapeForAppleScript(newName))
			} else {
				script = fmt.Sprintf(`tell application "Finder"
	try
		set sourceFile to (POSIX file "%s") as alias
		set parentFolder to (container of sourceFile) as alias
		set newFile to duplicate sourceFile to parentFolder
		return "Duplicated to: " & (POSIX path of (newFile as alias))
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path))
			}

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to duplicate file: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderCompressTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_compress",
		Description: "Compress (zip) a file or folder using Finder's built-in compression",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Full path to the file/folder to compress (required)"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)

			script := fmt.Sprintf(`tell application "Finder"
	try
		set targetFile to (POSIX file "%s") as alias
		set parentFolder to (container of targetFile) as alias
		set zipPath to (parentFolder as text) & (name of targetFile) & ".zip"
		-- Use shell to compress
		set posixPath to (POSIX path of targetFile)
		do shell script "ditto -ck --rsrc " & quoted form of posixPath & " " & quoted form of (posixPath & ".zip")
		return "Compressed to: " & posixPath & ".zip"
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path))
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to compress file: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderGetInfoTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_get_info",
		Description: "Get detailed info about a file or folder (size, kind, dates, permissions)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path": {Type: jsonschema.String, Description: "Full path to the file/folder (required)"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)

			script := fmt.Sprintf(`tell application "Finder"
	try
		set targetFile to (POSIX file "%s") as alias
		set infoStr to "File: " & (name of targetFile) & return
		set infoStr to infoStr & "Kind: " & (kind of targetFile) & return
		set infoStr to infoStr & "Path: " & (POSIX path of targetFile) & return
		try
			set infoStr to infoStr & "Size: " & ((size of targetFile) as text) & " bytes" & return
		end try
		set infoStr to infoStr & "Created: " & ((creation date of targetFile) as text) & return
		set infoStr to infoStr & "Modified: " & ((modification date of targetFile) as text) & return
		try
			set infoStr to infoStr & "Label: " & (label index of targetFile as text) & return
		end try
		try
			if (class of targetFile) is folder then
				set infoStr to infoStr & "Type: Folder" & return
				set itemCount to (count of (items of targetFile))
				set infoStr to infoStr & "Items: " & itemCount & return
			else
				set infoStr to infoStr & "Type: File" & return
			end if
		end try
		return infoStr
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path))
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get file info: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func finderTagTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_finder_tag",
		Description: "Add or remove Finder tags/labels from a file or folder (tag numbers: 0=none, 1=gray, 2=green, 3=purple, 4=blue, 5=yellow, 6=red, 7=orange)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":  {Type: jsonschema.String, Description: "Full path to the file/folder (required)"},
				"label": {Type: jsonschema.Integer, Description: "Label index 0-7 (0=remove, 1=gray, 2=green, 3=purple, 4=blue, 5=yellow, 6=red, 7=orange), default 0"},
			},
			Required: []string{"path"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			path, _ := args["path"].(string)
			label := 0
			if l, ok := args["label"].(float64); ok {
				label = int(l)
			}

			labelNames := map[int]string{
				0: "none", 1: "gray", 2: "green", 3: "purple",
				4: "blue", 5: "yellow", 6: "red", 7: "orange",
			}
			labelName := labelNames[label]
			if labelName == "" {
				labelName = "none"
			}

			script := fmt.Sprintf(`tell application "Finder"
	try
		set targetFile to (POSIX file "%s") as alias
		set label index of targetFile to %d
		return "Set label of " & (name of targetFile) & " to '%s'"
	on error errMsg
		return "Error: " & errMsg
	end try
end tell`, escapeForAppleScript(path), label, escapeForAppleScript(labelName))
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to tag file: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}
