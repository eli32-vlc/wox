package system

import (
	"context"
	"fmt"
	"strings"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

// GetMacOSAXTools returns MCP tools for macOS Accessibility (AX) tree inspection
// and GUI automation via System Events AppleScript. These tools allow the AI to
// read app UI hierarchies, click buttons, set text fields, and manipulate UI
// elements in any application that supports the Accessibility API.
func GetMacOSAXTools() []common.MCPTool {
	return []common.MCPTool{
		axGetProcessListTool(),
		axLaunchAppTool(),
		axFocusAppTool(),
		axGetFocusedElementTool(),
		axGetWindowElementsTool(),
		axGetElementTool(),
		axClickElementTool(),
		axSetTextTool(),
		axGetTextTool(),
		axShowMenuTool(),
		axScrollTool(),
		axGetElementTreeTool(),
	}
}

// --- AX Process Tools ---

func axGetProcessListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_process_list",
		Description: "List all running GUI applications (processes) that can be inspected via Accessibility API",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "System Events"
	set output to ""
	set processList to (every process where background only is false)
	repeat with p in processList
		set output to output & "  " & name of p & " (PID: " & (unix id of p as text) & ")" & return
	end repeat
	if output is "" then
		set output to "No GUI processes found."
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list processes: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axLaunchAppTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_launch_app",
		Description: "Launch an application by its name or bundle identifier",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName": {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Notes', 'System Settings') — required"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			script := fmt.Sprintf(`tell application "%s"
	activate
	return "Launched " & "%s"
end tell`, appName, appName)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to launch app '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axFocusAppTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_focus_app",
		Description: "Bring an already-running application to the foreground",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName": {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Notes') — required"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		set frontmost to true
	end tell
	return "Focused " & "%s"
end tell`, appName, appName)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to focus app '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- AX Element Query Tools ---

func axGetFocusedElementTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_focused_element",
		Description: "Get detailed properties of the currently focused UI element in the frontmost application",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "System Events"
	set frontApp to name of first process whose frontmost is true
	tell process frontApp
		set focusedEl to focused of window 1
		if focusedEl is not missing value then					set elRole to ""
					try
						set elRole to (value of attribute "AXRole" of focusedEl) as text
					end try
					set elDesc to ""
					try
						set elDesc to description of focusedEl
					end try
			set elValue to ""
			try
				set elValue to (value of focusedEl) as text
			end try
			set {elX, elY} to position of focusedEl
			set {elW, elH} to size of focusedEl
			set enabledStr to ""
			try
				if enabled of focusedEl then set enabledStr to "yes"
			end try
			return "App: " & frontApp & return & "Role: " & elRole & return & "Description: " & elDesc & return & "Value: " & elValue & return & "Position: (" & elX & ", " & elY & ")" & return & "Size: " & elW & "x" & elH & return & "Enabled: " & enabledStr
		else
			return "No focused element found in frontmost app: " & frontApp
		end if
	end tell
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get focused element: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axGetWindowElementsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_window_elements",
		Description: "List all UI elements in a window of a given application, including their roles and descriptions",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Finder') — required"},
				"windowIndex": {Type: jsonschema.Integer, Description: "Window index (1 = frontmost window, default 1)"},
				"maxDepth":    {Type: jsonschema.Integer, Description: "Maximum depth to traverse (default 2, max 5)"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			windowIdx := 1
			if w, ok := args["windowIndex"].(float64); ok {
				windowIdx = int(w)
			}
			maxDepth := 2
			if d, ok := args["maxDepth"].(float64); ok {
				maxDepth = int(d)
				if maxDepth > 5 {
					maxDepth = 5
				}
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		set output to ""
		try
			set targetWindow to window %d
			set output to my listElements(targetWindow, 0, %d)
		on error errMsg
			set output to "Error: " & errMsg
		end try
		return output
	end tell
end tell

on listElements(el, depth, maxDepth)
	if depth > maxDepth then return ""
	set indent to ""
	repeat depth times
		set indent to indent & "  "
	end repeat
	set output to ""
	try
		set elRole to "unknown"
		try
			set elRole to (value of attribute "AXRole" of el) as text
		end try
		set elDesc to ""
		try
			set elDesc to description of el
		end try
		set elTitle to ""
		try
			set elTitle to title of el
		end try
		set elName to ""
		try
			set elName to name of el
		end try
		
		set elSummary to indent & "[" & elRole & "]"
		if elTitle is not "" then
			set elSummary to elSummary & " title=""" & elTitle & """"
		end if
		if elName is not "" then
			set elSummary to elSummary & " name=""" & elName & """"
		end if
		if elDesc is not "" then
			set elSummary to elSummary & " desc=""" & elDesc & """"
		end if
		set output to output & elSummary & return
		
		try
			set children to every UI element of el
			repeat with child in children
				set output to output & my listElements(child, depth + 1, maxDepth)
			end repeat
		end try
	on error
		-- skip elements that can't be inspected
	end try
	return output
end listElements`, appName, windowIdx, maxDepth)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get window elements for '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axGetElementTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_element",
		Description: "Get detailed properties of a UI element in an application window by its description, title, or role path",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Finder') — required"},
				"description": {Type: jsonschema.String, Description: "Accessibility description of the element to find"},
				"title":       {Type: jsonschema.String, Description: "Title or name of the element to find"},
				"role":        {Type: jsonschema.String, Description: "AXRole to filter by (e.g. 'AXButton', 'AXTextField', 'AXStaticText')"},
				"index":       {Type: jsonschema.Integer, Description: "Element index if multiple match (1-based, default 1)"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			desc, _ := args["description"].(string)
			title, _ := args["title"].(string)
			role, _ := args["role"].(string)
			index := 1
			if i, ok := args["index"].(float64); ok {
				index = int(i)
			}

			// Build a filter clause to find the element
			var filters []string
			if desc != "" {
				filters = append(filters, fmt.Sprintf(`description is "%s"`, desc))
			}
			if title != "" {
				filters = append(filters, fmt.Sprintf(`title is "%s"`, title))
			}
			if role != "" {
				filters = append(filters, fmt.Sprintf(`(value of attribute "AXRole") is "%s"`, role))
			}

			filterClause := ""
			if len(filters) > 0 {
				filterClause = " whose " + strings.Join(filters, " and ")
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		set output to ""
		try
			set matchingElements to (every UI element of window 1%s)
			if (count of matchingElements) >= %d then
				set el to item %d of matchingElements
				set elRole to "?"
				try
					set elRole to (value of attribute "AXRole" of el) as text
				end try
				set elDesc to ""
				try
					set elDesc to description of el
				end try
				set elTitle to ""
				try
					set elTitle to title of el
				end try
				set elName to ""
				try
					set elName to name of el
				end try
				set elValue to ""
				try
					set elValue to (value of el) as text
				end try
				set enabledStr to "?"
				try
					if enabled of el then set enabledStr to "yes" else set enabledStr to "no"
				end try
				set focusedStr to "?"
				try
					if focused of el then set focusedStr to "yes" else set focusedStr to "no"
				end try
				set {elX, elY} to position of el
				set {elW, elH} to size of el
				set output to "Role: " & elRole & return
				set output to output & "Description: " & elDesc & return
				set output to output & "Title: " & elTitle & return
				set output to output & "Name: " & elName & return
				set output to output & "Value: " & elValue & return
				set output to output & "Position: (" & elX & ", " & elY & ")" & return
				set output to output & "Size: " & elW & "x" & elH & return
				set output to output & "Enabled: " & enabledStr & return
				set output to output & "Focused: " & focusedStr
			else
				set output to "No matching elements found (found " & (count of matchingElements) & " elements)"
			end if
		on error errMsg
			set output to "Error: " & errMsg
		end try
		return output
	end tell
end tell`, appName, filterClause, index, index)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get element in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axGetElementTreeTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_element_tree",
		Description: "Get the full accessibility element tree for a window (recursive depth-first traversal with role and description)",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari', 'Finder') — required"},
				"windowIndex": {Type: jsonschema.Integer, Description: "Window index (1 = frontmost, default 1)"},
				"maxDepth":    {Type: jsonschema.Integer, Description: "Maximum depth (default 3, max 8)"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			windowIdx := 1
			if w, ok := args["windowIndex"].(float64); ok {
				windowIdx = int(w)
			}
			maxDepth := 3
			if d, ok := args["maxDepth"].(float64); ok {
				maxDepth = int(d)
				if maxDepth > 8 {
					maxDepth = 8
				}
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		set output to ""
		try
			set targetWindow to window %d
			set output to my dumpTree(targetWindow, 0, %d)
		on error errMsg
			set output to "Error: " & errMsg
		end try
		return output
	end tell
end tell

on dumpTree(el, depth, maxDepth)
	if depth > maxDepth then return ""
	set indent to ""
	repeat depth times
		set indent to indent & "  "
	end repeat
	set output to ""
	try
		set elRole to "?"
		try
			set elRole to (value of attribute "AXRole" of el) as text
		end try
		set shortRole to ""
		if elRole starts with "AX" then
			set shortRole to text 3 thru -1 of elRole
		else
			set shortRole to elRole
		end if
		
		set elDesc to ""
		try
			set elDesc to description of el
		end try
		set elTitle to ""
		try
			set elTitle to title of el
		end try
		set elValue to ""
		try
			set elValue to (value of el) as text
		end try
		
		-- Build a compact summary line
		set lineStr to indent & shortRole
		if elTitle is not "" then
			set lineStr to lineStr & " """ & elTitle & """"
		end if
		if elDesc is not "" then
			set lineStr to lineStr & " [" & elDesc & "]"
		end if
		if elValue is not "" and shortRole is "TextField" then
			set lineStr to lineStr & " = """ & elValue & """"
		end if
		set output to output & lineStr & return
		
		-- Recurse into children
		try
			set children to every UI element of el
			repeat with child in children
				set output to output & my dumpTree(child, depth + 1, maxDepth)
			end repeat
		end try
	on error
		-- skip
	end try
	return output
end dumpTree`, appName, windowIdx, maxDepth)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get element tree for '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- AX Action Tools ---

func axClickElementTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_click_element",
		Description: "Click (press) a UI element in an application by its description, title, or path",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari') — required"},
				"description": {Type: jsonschema.String, Description: "Accessibility description of the element to click"},
				"title":       {Type: jsonschema.String, Description: "Title or name of the element to click"},
				"role":        {Type: jsonschema.String, Description: "AXRole filter (e.g. 'AXButton')"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			desc, _ := args["description"].(string)
			title, _ := args["title"].(string)
			role, _ := args["role"].(string)

			var filters []string
			if desc != "" {
				filters = append(filters, fmt.Sprintf(`description is "%s"`, desc))
			}
			if title != "" {
				filters = append(filters, fmt.Sprintf(`title is "%s"`, title))
			}
			if role != "" {
				shortRole := strings.TrimPrefix(role, "AX")
				filters = append(filters, fmt.Sprintf(`(value of attribute "AXRole") is "%s"`, "AX"+shortRole))
			}

			filterClause := ""
			if len(filters) > 0 {
				filterClause = " whose " + strings.Join(filters, " and ")
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set matchingElements to (every UI element of window 1%s)
			if (count of matchingElements) > 0 then
				set el to first item of matchingElements
				click el
				return "Clicked element"
			else
				return "No matching element found"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, filterClause)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to click element in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axSetTextTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_set_text",
		Description: "Set the value of a text field in an application",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari') — required"},
				"value":       {Type: jsonschema.String, Description: "Text to set in the field (required)"},
				"description": {Type: jsonschema.String, Description: "Description of the text field element"},
				"index":       {Type: jsonschema.Integer, Description: "Text field index if multiple (default 1)"},
			},
			Required: []string{"appName", "value"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			newValue, _ := args["value"].(string)
			desc, _ := args["description"].(string)
			index := 1
			if i, ok := args["index"].(float64); ok {
				index = int(i)
			}

			var script string
			if desc != "" {
				script = fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set matchingFields to (every text field of window 1 whose description is "%s")
			if (count of matchingFields) >= %d then
				set value of item %d of matchingFields to "%s"
				return "Set text field to: " & "%s"
			else
				return "No matching text field found"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, desc, index, index, newValue, newValue)
			} else {
				script = fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set allFields to (every text field of window 1)
			if (count of allFields) >= %d then
				set value of item %d of allFields to "%s"
				return "Set text field %d to: " & "%s"
			else
				return "Not enough text fields found (found " & (count of allFields) & ")"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, index, index, newValue, index, newValue)
			}

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to set text in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axGetTextTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_get_text",
		Description: "Get the current value/text from a text field or static text element in an application",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Safari') — required"},
				"description": {Type: jsonschema.String, Description: "Optional description of the element"},
				"index":       {Type: jsonschema.Integer, Description: "Text field index if multiple (default 1)"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			desc, _ := args["description"].(string)
			index := 1
			if i, ok := args["index"].(float64); ok {
				index = int(i)
			}

			var script string
			if desc != "" {
				script = fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set matchingElements to (every UI element of window 1 whose description is "%s" and (value of attribute "AXRole") is "AXTextField")
			if (count of matchingElements) >= %d then
				set elValue to (value of item %d of matchingElements) as text
				return elValue
			else
				return "No matching text field found"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, desc, index, index)
			} else {
				script = fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set allFields to (every text field of window 1)
			if (count of allFields) >= %d then
				set elValue to (value of item %d of allFields) as text
				return elValue
			else
				return "Not enough text fields found (found " & (count of allFields) & ")"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, index, index)
			}

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get text in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axShowMenuTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_show_menu",
		Description: "Show (right-click) context menu on a UI element in an application",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":     {Type: jsonschema.String, Description: "Application name (e.g. 'Finder') — required"},
				"description": {Type: jsonschema.String, Description: "Description of the element"},
				"title":       {Type: jsonschema.String, Description: "Title of the element"},
			},
			Required: []string{"appName"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			desc, _ := args["description"].(string)
			title, _ := args["title"].(string)

			var filters []string
			if desc != "" {
				filters = append(filters, fmt.Sprintf(`description is "%s"`, desc))
			}
			if title != "" {
				filters = append(filters, fmt.Sprintf(`title is "%s"`, title))
			}

			filterClause := ""
			if len(filters) > 0 {
				filterClause = " whose " + strings.Join(filters, " and ")
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		try
			set matchingElements to (every UI element of window 1%s)
			if (count of matchingElements) > 0 then
				set el to first item of matchingElements
				perform action "AXShowMenu" of el
				return "Showed context menu on element"
			else
				return "No matching element found"
			end if
		on error errMsg
			return "Error: " & errMsg
		end try
	end tell
end tell`, appName, filterClause)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to show menu in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func axScrollTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_ax_scroll",
		Description: "Scroll in a scroll area or list by a delta amount",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"appName":   {Type: jsonschema.String, Description: "Application name (e.g. 'Safari') — required"},
				"direction": {Type: jsonschema.String, Description: "Scroll direction: 'down', 'up', 'left', 'right' (required)"},
				"lines":     {Type: jsonschema.Integer, Description: "Number of lines/steps to scroll (default 1)"},
			},
			Required: []string{"appName", "direction"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			appName, _ := args["appName"].(string)
			direction, _ := args["direction"].(string)
			lines := 1
			if l, ok := args["lines"].(float64); ok {
				lines = int(l)
			}

			keyCode := 125 // down arrow
			switch strings.ToLower(direction) {
			case "up":
				keyCode = 126
			case "down":
				keyCode = 125
			case "left":
				keyCode = 123
			case "right":
				keyCode = 124
			}

			script := fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		set output to ""
		try
			repeat %d times
				key code %d
			end repeat
			set output to "Scrolled " & "%s" & " " & %d & " times"
		on error errMsg
			set output to "Error: " & errMsg
		end try
		return output
	end tell
end tell`, appName, lines, keyCode, direction, lines)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to scroll in '%s': %s", appName, err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}
