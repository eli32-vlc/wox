package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

// GetMacOSAppTools returns MCP tools for interacting with macOS built-in apps
// via AppleScript (osascript). These tools allow the AI to read, create, and
// manage data in Calendar, Reminders, Mail, Notes, Contacts, Messages, Maps,
// Music, and Safari.
func GetMacOSAppTools() []common.MCPTool {
	return []common.MCPTool{
		// Calendar
		calendarListEventsTool(),
		calendarCreateEventTool(),
		calendarDeleteEventTool(),
		calendarListCalendarsTool(),

		// Reminders
		remindersListTool(),
		remindersCreateTool(),
		remindersCompleteTool(),
		remindersDeleteTool(),

		// Mail
		mailListMessagesTool(),
		mailReadMessageTool(),
		mailSendTool(),
		mailSearchTool(),
		mailListAccountsTool(),

		// Notes
		notesListTool(),
		notesReadTool(),
		notesCreateTool(),
		notesDeleteTool(),

		// Contacts
		contactsSearchTool(),
		contactsGetTool(),
		contactsListGroupsTool(),

		// Messages
		messagesSendTool(),
		messagesListRecentTool(),

		// Maps
		mapsSearchTool(),
		mapsGetDirectionsTool(),

		// Music
		musicCurrentTrackTool(),
		musicPlayPauseTool(),
		musicNextTrackTool(),
		musicPreviousTrackTool(),
		musicPlaylistsTool(),

		// Safari
		safariGetBookmarksTool(),
		safariListTabsTool(),
		safariOpenURLTool(),
	}
}

// --- Calendar Tools ---

func calendarListEventsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_calendar_list_events",
		Description: "List calendar events for a date range. Defaults to today if no dates provided.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"days":     {Type: jsonschema.Integer, Description: "Number of days from today to list (default 1). Use negative for past."},
				"calendar": {Type: jsonschema.String, Description: "Optional calendar name to filter by (e.g. 'Work', 'Home')"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			days := 1
			if d, ok := args["days"].(float64); ok {
				days = int(d)
			}
			calFilter, _ := args["calendar"].(string)

			script := fmt.Sprintf(`set startDate to current date
set hours of startDate to 0
set minutes of startDate to 0
set seconds of startDate to 0
set endDate to startDate + (%d * days)
tell application "Calendar"
	set output to ""
	set matchingCalendars to every calendar
	if "%s" is not "" then
		set matchingCalendars to (every calendar whose title is "%s")
	end if
	repeat with cal in matchingCalendars
		set eventList to (every event of cal whose start date ≥ startDate and start date < endDate)
		repeat with ev in eventList
			set startStr to (start date of ev) as text
			set endStr to (end date of ev) as text
			set output to output & "Event: " & summary of ev & return
			set output to output & "  Calendar: " & title of cal & return
			set output to output & "  Start: " & startStr & return
			set output to output & "  End: " & endStr & return
			if location of ev is not "" then
				set output to output & "  Location: " & location of ev & return
			end if
			if description of ev is not "" then
				set output to output & "  Description: " & description of ev & return
			end if
			set output to output & "---" & return
		end repeat
	end repeat
	if output is "" then
		set output to "No events found in the specified date range."
	end if
	return output
end tell`, days, calFilter, calFilter)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list calendar events: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func calendarCreateEventTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_calendar_create_event",
		Description: "Create a new calendar event.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title":       {Type: jsonschema.String, Description: "Event title (required)"},
				"startDate":   {Type: jsonschema.String, Description: "Start date/time in format like '2025-01-15 14:00' or 'tomorrow at 2pm'"},
				"endDate":     {Type: jsonschema.String, Description: "End date/time in format like '2025-01-15 15:00'"},
				"location":    {Type: jsonschema.String, Description: "Optional event location"},
				"notes":       {Type: jsonschema.String, Description: "Optional event notes/description"},
				"calendar":    {Type: jsonschema.String, Description: "Calendar name (default: first available calendar)"},
				"alarmMinutes": {Type: jsonschema.Integer, Description: "Optional alarm reminder in minutes before the event"},
			},
			Required: []string{"title", "startDate", "endDate"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			startDate, _ := args["startDate"].(string)
			endDate, _ := args["endDate"].(string)
			location, _ := args["location"].(string)
			notes, _ := args["notes"].(string)
			calName, _ := args["calendar"].(string)
			alarmMin := 0
			if a, ok := args["alarmMinutes"].(float64); ok {
				alarmMin = int(a)
			}

			// Build the AppleScript piece by piece to avoid fmt.Sprintf spanning conditionals
			script := fmt.Sprintf(`tell application "Calendar"
	set targetCal to first calendar
	if "%s" is not "" then
		try
			set targetCal to (first calendar whose title is "%s")
		end try
	end if
	set newEvent to make new event at end of targetCal with properties {summary:"%s", start date:(date "%s"), end date:(date "%s")`, calName, calName, title, startDate, endDate)
			if location != "" {
				script += fmt.Sprintf(`, location:"%s"`, location)
			}
			if notes != "" {
				script += fmt.Sprintf(`, description:"%s"`, notes)
			}
			script += fmt.Sprintf(`}
	if %d > 0 then
		tell newEvent to make new display alarm at end with properties {trigger interval:-%d}
	end if
	return "Created event: " & summary of newEvent
end tell`, alarmMin, alarmMin)

			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to create event: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func calendarDeleteEventTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_calendar_delete_event",
		Description: "Delete a calendar event by its title and optional date.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title": {Type: jsonschema.String, Description: "Event title to delete (required)"},
				"date":  {Type: jsonschema.String, Description: "Optional date string to narrow search, e.g. '2025-01-15'"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			dateStr, _ := args["date"].(string)

			script := fmt.Sprintf(`tell application "Calendar"
	set deletedCount to 0
	repeat with cal in (every calendar)
		set matchingEvents to (every event whose summary is "%s")
		if "%s" is not "" then
			set matchingEvents to (every event whose summary is "%s" and (start date) contains (date "%s"))
		end if
		repeat with ev in matchingEvents
			delete ev
			set deletedCount to deletedCount + 1
		end repeat
	end repeat
	return "Deleted " & deletedCount & " event(s) with title '" & "%s" & "'"
end tell`, title, dateStr, title, dateStr, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to delete event: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func calendarListCalendarsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_calendar_list_calendars",
		Description: "List all calendars (groups) in the Calendar app.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Calendar"
	set output to ""
	repeat with cal in (every calendar)
		set output to output & title of cal & " (" & (count of events of cal) & " events)" & return
	end repeat
	if output is "" then
		set output to "No calendars found."
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list calendars: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Reminders Tools ---

func remindersListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_reminders_list",
		Description: "List reminders. Optionally filter by list name and completion status.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"list":     {Type: jsonschema.String, Description: "Optional reminder list name to filter by"},
				"showCompleted": {Type: jsonschema.Boolean, Description: "Whether to show completed reminders (default false)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			listFilter, _ := args["list"].(string)
			showCompleted := false
			if s, ok := args["showCompleted"].(bool); ok {
				showCompleted = s
			}

			completedFilter := "whose completed is false"
			if showCompleted {
				completedFilter = ""
			}

			script := fmt.Sprintf(`tell application "Reminders"
	set output to ""
	set targetLists to (every list)
	if "%s" is not "" then
		set targetLists to (every list whose name is "%s")
	end if
	repeat with lst in targetLists
		set output to output & "List: " & name of lst & return
		set remindersToCheck to (every reminder of lst %s)
		repeat with r in remindersToCheck
			set output to output & "  ☐ " & name of r & return
			if body of r is not "" then
				set output to output & "    Note: " & body of r & return
			end if
			if due date of r is not missing value then
				set output to output & "    Due: " & (due date of r as text) & return
			end if
			if priority of r > 0 then
				set output to output & "    Priority: " & priority of r & return
			end if
		end repeat
		set output to output & "---" & return
	end repeat
	if output is "" then
		set output to "No reminders found."
	end if
	return output
end tell`, listFilter, listFilter, completedFilter)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list reminders: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func remindersCreateTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_reminders_create",
		Description: "Create a new reminder.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title":      {Type: jsonschema.String, Description: "Reminder title (required)"},
				"notes":      {Type: jsonschema.String, Description: "Optional notes/body for the reminder"},
				"dueDate":    {Type: jsonschema.String, Description: "Optional due date, e.g. 'tomorrow at 9am'"},
				"priority":   {Type: jsonschema.Integer, Description: "Optional priority (1=high, 5=medium, 9=low)"},
				"list":        {Type: jsonschema.String, Description: "Optional list name (default: first available list)"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			notes, _ := args["notes"].(string)
			dueDate, _ := args["dueDate"].(string)
			priority := 0
			if p, ok := args["priority"].(float64); ok {
				priority = int(p)
			}
			listName, _ := args["list"].(string)

			script := fmt.Sprintf(`tell application "Reminders"
	set targetList to first list
	if "%s" is not "" then
		try
			set targetList to (first list whose name is "%s")
		end try
	end if
	set newReminder to make new reminder at end of targetList with properties {name:"%s", body:"%s"`, listName, listName, title, notes)
			if dueDate != "" {
				script += fmt.Sprintf(`, due date:(date "%s")`, dueDate)
			}
			if priority > 0 {
				script += fmt.Sprintf(`, priority:%d`, priority)
			}
			script += `}
	return "Created reminder: " & name of newReminder
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to create reminder: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func remindersCompleteTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_reminders_complete",
		Description: "Mark a reminder as completed.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title": {Type: jsonschema.String, Description: "Reminder title to mark as completed (required)"},
				"list":  {Type: jsonschema.String, Description: "Optional list name to narrow the search"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			listName, _ := args["list"].(string)

			script := fmt.Sprintf(`tell application "Reminders"
	set targetLists to (every list)
	if "%s" is not "" then
		set targetLists to (every list whose name is "%s")
	end if
	set completedCount to 0
	repeat with lst in targetLists
		set matchingReminders to (every reminder of lst whose name is "%s")
		repeat with r in matchingReminders
			set completed of r to true
			set completedCount to completedCount + 1
		end repeat
	end repeat
	return "Completed " & completedCount & " reminder(s) with title '" & "%s" & "'"
end tell`, listName, listName, title, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to complete reminder: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func remindersDeleteTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_reminders_delete",
		Description: "Delete a reminder permanently.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title": {Type: jsonschema.String, Description: "Reminder title to delete (required)"},
				"list":  {Type: jsonschema.String, Description: "Optional list name to narrow the search"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			listName, _ := args["list"].(string)

			script := fmt.Sprintf(`tell application "Reminders"
	set targetLists to (every list)
	if "%s" is not "" then
		set targetLists to (every list whose name is "%s")
	end if
	set deletedCount to 0
	repeat with lst in targetLists
		set matchingReminders to (every reminder of lst whose name is "%s")
		repeat with r in matchingReminders
			delete r
			set deletedCount to deletedCount + 1
		end repeat
	end repeat
	return "Deleted " & deletedCount & " reminder(s) with title '" & "%s" & "'"
end tell`, listName, listName, title, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to delete reminder: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Mail Tools ---

func mailListMessagesTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mail_list_messages",
		Description: "List email messages in the inbox. Returns sender, subject, and date.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"account": {Type: jsonschema.String, Description: "Optional mail account name to filter by"},
				"mailbox": {Type: jsonschema.String, Description: "Optional mailbox name (default: 'INBOX')"},
				"limit":   {Type: jsonschema.Integer, Description: "Maximum number of messages to return (default 10)"},
				"unreadOnly": {Type: jsonschema.Boolean, Description: "Only show unread messages (default false)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			account, _ := args["account"].(string)
			mailbox := "INBOX"
			if m, ok := args["mailbox"].(string); ok && m != "" {
				mailbox = m
			}
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			unreadOnly := false
			if u, ok := args["unreadOnly"].(bool); ok {
				unreadOnly = u
			}

			script := fmt.Sprintf(`tell application "Mail"
	set output to ""
	set accountFilter to missing value
	if "%s" is not "" then
		try
			set accountFilter to (first account whose name is "%s")
		end try
	end if
	
	set targetAccounts to (every account)
	if accountFilter is not missing value then
		set targetAccounts to {accountFilter}
	end if
	
	repeat with acc in targetAccounts
		try
			set targetMailbox to (first mailbox of acc whose name is "%s")
			set allMessages to (every message of targetMailbox)
			if %d > 0 and (count allMessages) > %d then
				set allMessages to items 1 through %d of allMessages
			end if
			repeat with msg in allMessages					if %s then
						if read status of msg is true then
							next repeat
						end if
					end if
				set output to output & "From: " & sender of msg & return
				set output to output & "Subject: " & subject of msg & return
				set output to output & "Date: " & (date received of msg as text) & return
				set output to output & "---" & return
			end repeat
		end try
	end repeat
	if output is "" then
		set output to "No messages found."
	end if
	return output
end tell`, account, account, mailbox, limit, limit, limit, boolToScript(unreadOnly))
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list mail messages: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func mailReadMessageTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mail_read_message",
		Description: "Read the full content of an email message by subject keyword or sender.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"subject": {Type: jsonschema.String, Description: "Subject keyword to search for (required)"},
				"sender":  {Type: jsonschema.String, Description: "Optional sender email to narrow search"},
			},
			Required: []string{"subject"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			subject, _ := args["subject"].(string)
			sender, _ := args["sender"].(string)
			if sender == "" {
				sender = "*"
			}

			script := fmt.Sprintf(`tell application "Mail"
	set output to ""
	repeat with acc in (every account)
		try
			set inbox to first mailbox of acc whose name is "INBOX"
			set matchingMessages to (every message of inbox whose subject contains "%s" and sender contains "%s")
			repeat with msg in matchingMessages
				set output to output & "From: " & sender of msg & return
				set output to output & "Subject: " & subject of msg & return
				set output to output & "Date: " & (date received of msg as text) & return
				set output to output & "---" & return
				set output to output & content of msg & return
				set output to output & "===" & return
			end repeat
		end try
	end repeat
	if output is "" then
		set output to "No matching messages found."
	end if
	return output
end tell`, subject, sender)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read mail message: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func mailSendTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mail_send",
		Description: "Send an email using the Mail app.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"to":      {Type: jsonschema.String, Description: "Recipient email address(es), comma-separated (required)"},
				"subject": {Type: jsonschema.String, Description: "Email subject (required)"},
				"body":    {Type: jsonschema.String, Description: "Email body text (required)"},
				"cc":      {Type: jsonschema.String, Description: "Optional CC recipient(s)"},
				"bcc":     {Type: jsonschema.String, Description: "Optional BCC recipient(s)"},
			},
			Required: []string{"to", "subject", "body"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			to, _ := args["to"].(string)
			subject, _ := args["subject"].(string)
			body, _ := args["body"].(string)
			cc, _ := args["cc"].(string)
			bcc, _ := args["bcc"].(string)

			script := fmt.Sprintf(`tell application "Mail"
	set newMessage to make new outgoing message with properties {subject:"%s", content:"%s", visible:true}
	tell newMessage
		make new to recipient at end of to recipients with properties {address:"%s"}`, subject, body, to)
			if cc != "" {
				script += fmt.Sprintf(`
		make new cc recipient at end of cc recipients with properties {address:"%s"}`, cc)
			}
			if bcc != "" {
				script += fmt.Sprintf(`
		make new bcc recipient at end of bcc recipients with properties {address:"%s"}`, bcc)
			}
			script += fmt.Sprintf(`
	end tell
	send newMessage
	return "Email sent to " & "%s" & " with subject: " & "%s"
end tell`, to, subject)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to send email: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func mailSearchTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mail_search",
		Description: "Search emails across all mailboxes by keyword.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"keyword": {Type: jsonschema.String, Description: "Keyword to search in subject and content (required)"},
				"limit":   {Type: jsonschema.Integer, Description: "Maximum results (default 10)"},
			},
			Required: []string{"keyword"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			keyword, _ := args["keyword"].(string)
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Mail"
	set output to ""
	set foundCount to 0
	repeat with acc in (every account)
		try
			set inbox to first mailbox of acc whose name is "INBOX"
			set matchingMessages to (every message of inbox whose subject contains "%s" or content contains "%s")
			repeat with msg in matchingMessages
				if foundCount ≥ %d then exit repeat
				set output to output & "From: " & sender of msg & return
				set output to output & "Subject: " & subject of msg & return
				set output to output & "Date: " & (date received of msg as text) & return
				set output to output & "---" & return
				set foundCount to foundCount + 1
			end repeat
		end try
	end repeat
	if output is "" then
		set output to "No messages found for '" & "%s" & "'"
	end if
	return output
end tell`, keyword, keyword, limit, keyword)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to search mail: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func mailListAccountsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_mail_list_accounts",
		Description: "List all configured mail accounts.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Mail"
	set output to ""
	repeat with acc in (every account)
		set output to output & "Account: " & name of acc & return
		set output to output & "  Email: " & (first email address of acc) & return
		try
			set output to output & "  Mailboxes: " & (count of mailboxes of acc) & return
		end try
		set output to output & "---" & return
	end repeat
	if output is "" then
		set output to "No mail accounts configured."
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list mail accounts: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Notes Tools ---

func notesListTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_notes_list",
		Description: "List all notes with their titles and folders.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"folder": {Type: jsonschema.String, Description: "Optional folder name to filter by"},
				"limit":  {Type: jsonschema.Integer, Description: "Maximum notes to return (default 20)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			folder, _ := args["folder"].(string)
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Notes"
	set output to ""
	set targetFolders to (every folder)
	if "%s" is not "" then
		set targetFolders to (every folder whose name is "%s")
	end if
	set noteCount to 0
	repeat with f in targetFolders
		repeat with n in (every note of f)
			if noteCount ≥ %d then exit repeat
			set output to output & "Note: " & name of n & return
			set output to output & "  Folder: " & name of f & return
			set output to output & "---" & return
			set noteCount to noteCount + 1
		end repeat
	end repeat
	if output is "" then
		set output to "No notes found."
	end if
	return output
end tell`, folder, folder, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list notes: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func notesReadTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_notes_read",
		Description: "Read the full content of a note by its title.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title": {Type: jsonschema.String, Description: "Note title to read (required)"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)

			script := fmt.Sprintf(`tell application "Notes"
	set output to ""
	repeat with f in (every folder)
		set matchingNotes to (every note of f whose name is "%s")
		repeat with n in matchingNotes
			set output to output & "Title: " & name of n & return
			set output to output & "Folder: " & name of f & return
			set output to output & "---" & return
			set output to output & body of n & return
		end repeat
	end repeat
	if output is "" then
		set output to "No note found with title '" & "%s" & "'"
	end if
	return output
end tell`, title, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to read note: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func notesCreateTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_notes_create",
		Description: "Create a new note.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title":  {Type: jsonschema.String, Description: "Note title (required)"},
				"body":   {Type: jsonschema.String, Description: "Note body/content (required)"},
				"folder": {Type: jsonschema.String, Description: "Optional folder name (default: first available folder)"},
			},
			Required: []string{"title", "body"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)
			body, _ := args["body"].(string)
			folder, _ := args["folder"].(string)

			script := fmt.Sprintf(`tell application "Notes"
	set targetFolder to first folder
	if "%s" is not "" then
		try
			set targetFolder to (first folder whose name is "%s")
		end try
	end if
	make new note at targetFolder with properties {name:"%s", body:"%s"}
	return "Created note: " & "%s" & " in folder " & name of targetFolder
end tell`, folder, folder, title, body, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to create note: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func notesDeleteTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_notes_delete",
		Description: "Delete a note by its title.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"title": {Type: jsonschema.String, Description: "Note title to delete (required)"},
			},
			Required: []string{"title"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			title, _ := args["title"].(string)

			script := fmt.Sprintf(`tell application "Notes"
	set deletedCount to 0
	repeat with f in (every folder)
		set matchingNotes to (every note of f whose name is "%s")
		repeat with n in matchingNotes
			delete n
			set deletedCount to deletedCount + 1
		end repeat
	end repeat
	return "Deleted " & deletedCount & " note(s) with title '" & "%s" & "'"
end tell`, title, title)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to delete note: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Contacts Tools ---

func contactsSearchTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_contacts_search",
		Description: "Search contacts by name, email, or phone number.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {Type: jsonschema.String, Description: "Search term to match against name, email, or phone (required)"},
				"limit": {Type: jsonschema.Integer, Description: "Maximum results (default 10)"},
			},
			Required: []string{"query"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			query, _ := args["query"].(string)
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Contacts"
	set output to ""
	set matchingPeople to (every person whose name contains "%s")
	set foundCount to 0
	repeat with p in matchingPeople
		if foundCount ≥ %d then exit repeat
		set output to output & "Name: " & name of p & return
		try
			set output to output & "  Email: " & value of (first email of p) & return
		end try
		try
			set output to output & "  Phone: " & value of (first phone of p) & return
		end try
		set output to output & "---" & return
		set foundCount to foundCount + 1
	end repeat
	if output is "" then
		set output to "No contacts found for '" & "%s" & "'"
	end if
	return output
end tell`, query, limit, query)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to search contacts: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func contactsGetTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_contacts_get",
		Description: "Get full details of a contact by their full name.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Full or partial name of the contact (required)"},
			},
			Required: []string{"name"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			name, _ := args["name"].(string)

			script := fmt.Sprintf(`tell application "Contacts"
	set output to ""
	set matchingPeople to (every person whose name contains "%s")
	repeat with p in matchingPeople
		set output to output & "Name: " & name of p & return
		repeat with e in (every email of p)
			set output to output & "  Email: " & value of e & " (" & label of e & ")" & return
		end repeat
		repeat with ph in (every phone of p)
			set output to output & "  Phone: " & value of ph & " (" & label of ph & ")" & return
		end repeat
		repeat with a in (every address of p)
			try
				set output to output & "  Address: " & ((value of a) as text) & return
			end try
		end repeat
		set output to output & "---" & return
	end repeat
	if output is "" then
		set output to "No contact found with name '" & "%s" & "'"
	end if
	return output
end tell`, name, name)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get contact: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func contactsListGroupsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_contacts_list_groups",
		Description: "List all contact groups.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Contacts"
	set output to ""
	repeat with g in (every group)
		set output to output & "Group: " & name of g & return
		set memberCount to (count of (every person of g))
		set output to output & "  Members: " & memberCount & return
	end repeat
	if output is "" then
		set output to "No contact groups found."
	end if
	return output
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list contact groups: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Messages Tools ---

func messagesSendTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_messages_send",
		Description: "Send an iMessage or SMS to a contact.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"recipient": {Type: jsonschema.String, Description: "Recipient phone number, email, or contact name (required)"},
				"message":   {Type: jsonschema.String, Description: "Message text to send (required)"},
			},
			Required: []string{"recipient", "message"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			recipient, _ := args["recipient"].(string)
			message, _ := args["message"].(string)

			script := fmt.Sprintf(`tell application "Messages"
	set targetService to (first service whose service type is iMessage)
	set targetBuddy to participant "%s" of targetService
	send "%s" to targetBuddy
	return "Message sent to " & "%s"
end tell`, recipient, message, recipient)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to send message: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func messagesListRecentTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_messages_list_recent",
		Description: "List recent message conversations. Returns sender and snippet.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum conversations to return (default 5)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 5
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Messages"
	set output to ""
	set chatCount to 0
	repeat with c in (every chat)
		if chatCount ≥ %d then exit repeat
		try
			set output to output & "Buddy: " & name of participant 1 of c & return
			set output to output & "  Last message: " & (text of last text of c) & return
		end try
		set output to output & "---" & return
		set chatCount to chatCount + 1
	end repeat
	if output is "" then
		set output to "No recent messages found."
	end if
	return output
end tell`, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list recent messages: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Maps Tools ---

func mapsSearchTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_maps_search",
		Description: "Search for a location using Apple Maps.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {Type: jsonschema.String, Description: "Location to search for, e.g. 'Coffee near me' or '123 Main St, NYC' (required)"},
			},
			Required: []string{"query"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			query, _ := args["query"].(string)

			// Maps doesn't have a great AppleScript API, so we open Maps with the search
			script := fmt.Sprintf(`tell application "Maps"
	search for "%s"
	set output to ""
	try
		set searchResults to (every place of first result window)
		repeat with p in searchResults
			set output to output & "Name: " & name of p & return
			try
				set output to output & "  Address: " & full address of p & return
			end try
			try
				set output to output & "  Phone: " & phone of p & return
			end try
			set output to output & "---" & return
		end repeat
	on error errMsg
		set output to "Search completed. View results in Maps app."
	end try
	if output is "" then
		set output to "Search completed. View results in Maps app."
	end if
	return output
end tell`, query)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to search maps: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func mapsGetDirectionsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_maps_get_directions",
		Description: "Get directions from one location to another using Apple Maps.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"from": {Type: jsonschema.String, Description: "Starting location or 'Current Location' (required)"},
				"to":   {Type: jsonschema.String, Description: "Destination location (required)"},
			},
			Required: []string{"from", "to"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			from, _ := args["from"].(string)
			to, _ := args["to"].(string)

			script := fmt.Sprintf(`tell application "Maps"
	set output to ""
	try
		open location "https://maps.apple.com/?saddr=%s&daddr=%s"
		set output to "Opened Maps with directions from '" & "%s" & "' to '" & "%s" & "'"
	on error errMsg
		set output to "Failed to get directions: " & errMsg
	end try
	return output
end tell`, from, to, from, to)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get directions: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Music Tools ---

func musicCurrentTrackTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_music_current_track",
		Description: "Get information about the currently playing track in Music.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Music"
	if player state is playing then
		set trackName to name of current track
		set artistName to artist of current track
		set albumName to album of current track
		set durationSec to duration of current track
		set playerPos to player position
		set output to "Now Playing:" & return
		set output to output & "  Track: " & trackName & return
		set output to output & "  Artist: " & artistName & return
		set output to output & "  Album: " & albumName & return
		set output to output & "  Duration: " & (durationSec as text) & "s" & return
		set output to output & "  Position: " & (playerPos as text) & "s" & return
		if sound volume is not missing value then
			set output to output & "  Volume: " & sound volume & return
		end if
		return output
	else
		return "Music is not currently playing."
	end if
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get current track: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func musicPlayPauseTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_music_play_pause",
		Description: "Toggle play/pause in the Music app.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Music"
	playpause
	if player state is playing then
		return "Music is now playing"
	else
		return "Music is now paused"
	end if
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to toggle play/pause: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func musicNextTrackTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_music_next_track",
		Description: "Skip to the next track in the Music app.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Music"
	next track
	return "Skipped to next track"
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to skip track: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func musicPreviousTrackTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_music_previous_track",
		Description: "Go back to the previous track in the Music app.",
		Parameters:  jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{}},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			script := `tell application "Music"
	previous track
	return "Went to previous track"
end tell`
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to go to previous track: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func musicPlaylistsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_music_playlists",
		Description: "List all playlists in the Music app.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum playlists to return (default 20)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Music"
	set output to ""
	set plCount to 0
	repeat with pl in (every user playlist)
		if plCount ≥ %d then exit repeat
		set output to output & "Playlist: " & name of pl & return
		set output to output & "  Tracks: " & (count of (every track of pl)) & return
		set output to output & "  Duration: " & (duration of pl as text) & "s" & return
		set output to output & "---" & return
		set plCount to plCount + 1
	end repeat
	if output is "" then
		set output to "No playlists found."
	end if
	return output
end tell`, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list playlists: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Safari Tools ---

func safariGetBookmarksTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_safari_bookmarks",
		Description: "Get bookmarks from Safari.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum bookmarks to return (default 20)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Safari"
	set output to ""
	set totalBookmarks to 0
	tell bookmarks bar
		repeat with c in (every bookmark item)
			if totalBookmarks ≥ %d then exit repeat
			if class of c is bookmark item folder then
				set output to output & "Folder: " & name of c & return
			else
				set output to output & "Bookmark: " & name of c & return
				try
					set output to output & "  URL: " & URL of c & return
				end try
				set totalBookmarks to totalBookmarks + 1
			end if
		end repeat
	end tell
	if output is "" then
		set output to "No bookmarks found."
	end if
	return output
end tell`, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to get bookmarks: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func safariListTabsTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_safari_tabs",
		Description: "List all open tabs in Safari windows.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"limit": {Type: jsonschema.Integer, Description: "Maximum tabs to return (default 20)"},
			},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			limit := 20
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}

			script := fmt.Sprintf(`tell application "Safari"
	set output to ""
	set tabCount to 0
	repeat with w in (every window)
		repeat with t in (every tab of w)
			if tabCount ≥ %d then exit repeat
			set output to output & "Tab: " & name of t & return
			set output to output & "  URL: " & URL of t & return
			set output to output & "---" & return
			set tabCount to tabCount + 1
		end repeat
	end repeat
	if output is "" then
		set output to "No open tabs found."
	end if
	return output
end tell`, limit)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to list tabs: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

func safariOpenURLTool() common.MCPTool {
	return common.MCPTool{
		Name:        "macos_safari_open_url",
		Description: "Open a URL in a new Safari tab.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"url": {Type: jsonschema.String, Description: "URL to open in Safari (required)"},
			},
			Required: []string{"url"},
		},
		Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
			url, _ := args["url"].(string)

			script := fmt.Sprintf(`tell application "Safari"
	open location "%s"
	activate
	return "Opened URL in Safari: " & "%s"
end tell`, url, url)
			out, err := runAppleScript(script)
			if err != nil {
				return common.Conversation{}, fmt.Errorf("failed to open URL in Safari: %s", err.Error())
			}
			return common.Conversation{Role: common.ConversationRoleAssistant, Text: strings.TrimSpace(out)}, nil
		},
		ServerConfig: &common.AIChatMCPServerConfig{Name: "macos_system"},
	}
}

// --- Helpers ---

// runAppleScript executes an AppleScript via osascript and returns stdout.
func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			errMsg := string(exitErr.Stderr)
			if errMsg == "" {
				errMsg = err.Error()
			}
			return "", fmt.Errorf("osascript failed: %s", errMsg)
		}
		return "", fmt.Errorf("osascript failed: %s", err.Error())
	}
	if len(out) == 0 {
		return "", nil
	}
	// Filter non-printable chars
	var sb strings.Builder
	for _, r := range string(out) {
		if r == '\n' || r == '\t' || r >= 32 {
			sb.WriteRune(r)
		}
	}
	return sb.String(), nil
}

// boolToScript converts a Go bool to an AppleScript-compatible string.
func boolToScript(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
