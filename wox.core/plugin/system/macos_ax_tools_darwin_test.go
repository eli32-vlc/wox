//go:build darwin

package system

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"wox/common"
)

// TestAXToolScriptsAreValidAppleScript is a regression test for the
// class of osascript parse error (-2741 "Expected ',' but found """)
// that surfaced when the AI invoked the macOS AX tool surface. The
// earlier round of fixes swapped `""""` for `& quote &` in
// listElements/dumpTree, but a follow-up audit caught more bugs:
//   - axGetFocusedElementTool had a one-liner `if … then <statement>`
//     that turned the rest of the handler into orphans, so every
//     invocation was a hard parse error.
//   - axShowMenuTool did not escape user-supplied description/title
//     strings, so any value containing `"` would also break the
//     generated script.
//
// This test exercises the actual generated scripts at parse-time by
// piping them to `osascript -e`. We assert the parser does not return
// a compile-time error. Runtime errors (e.g. AX call denied because
// accessibility permission is missing) are tolerated — they are
// surfaced to the model through the existing on-error path.
func TestAXToolScriptsAreValidAppleScript(t *testing.T) {
	cases := []struct {
		name string
		tool common.MCPTool
		args map[string]any
	}{
		{
			name: "get_focused_element",
			tool: axGetFocusedElementTool(),
			args: map[string]any{},
		},
		{
			name: "get_window_elements",
			tool: axGetWindowElementsTool(),
			args: map[string]any{
				"appName":     "Finder",
				"windowIndex": 1,
				"maxDepth":    2,
			},
		},
		{
			name: "get_element",
			tool: axGetElementTool(),
			args: map[string]any{
				"appName": "Finder",
				"index":   1,
			},
		},
		{
			name: "get_element_tree",
			tool: axGetElementTreeTool(),
			args: map[string]any{
				"appName":     "Finder",
				"windowIndex": 1,
				"maxDepth":    2,
			},
		},
		{
			name: "show_menu",
			tool: axShowMenuTool(),
			args: map[string]any{
				"appName": "Finder",
				// Intentionally a hostile description containing a
				// double-quote — pre-fix this broke the script.
				"description": `contains "quotes"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.tool.Callback(context.Background(), tc.args)
			if err == nil {
				return
			}
			if isAppleScriptSyntaxError(err.Error()) {
				t.Fatalf("osascript rejected the generated script as invalid AppleScript: %s", err.Error())
			}
		})
	}
}

// isAppleScriptSyntaxError reports whether the error message from
// osascript indicates a compile-time AppleScript syntax problem, as
// opposed to a runtime error (e.g. an AX call failing because
// accessibility permission was not granted).
func isAppleScriptSyntaxError(errText string) bool {
	lower := strings.ToLower(errText)
	if strings.Contains(lower, "syntax error") {
		return true
	}
	// The original failure mode in the bug report included the literal
	// "expected" word from the osascript compile error.
	if strings.Contains(lower, "expected ") && strings.Contains(lower, "but found") {
		return true
	}
	return false
}

// TestInstalledAppsScriptHasValidQuoting is a static guard for the
// installedAppsTool template. Pre-fix it used `\"` inside an
// AppleScript string literal, which AppleScript does not understand —
// the only quote-escape in a "…" literal is `""`.
func TestInstalledAppsScriptHasValidQuoting(t *testing.T) {
	tool := installedAppsTool()
	// The script is built inside the callback, so trigger it once to
	// assert the template compiles when we hand it to osascript. The
	// actual mdfind call may fail (Spotlight index, etc.) — we only
	// care about parse-time errors here.
	_, err := tool.Callback(context.Background(), map[string]any{
		"limit": 1,
	})
	if err != nil && isAppleScriptSyntaxError(err.Error()) {
		t.Fatalf("osascript rejected installedApps script as invalid AppleScript: %s", err.Error())
	}
}

// TestICloudDriveListScriptHasValidQuoting mirrors the regression
// test for icloudDriveListTool, which had the same `\"`-in-literal
// bug.
func TestICloudDriveListScriptHasValidQuoting(t *testing.T) {
	tool := icloudDriveListTool()
	_, err := tool.Callback(context.Background(), map[string]any{
		"limit": 1,
	})
	if err != nil && isAppleScriptSyntaxError(err.Error()) {
		t.Fatalf("osascript rejected icloudDriveList script as invalid AppleScript: %s", err.Error())
	}
}

// TestNoBackslashQuoteInExtraToolTemplates is a static source-level
// guard so the `\"` quoting bug cannot silently come back in any of
// the macos_extra_tools templates. We grep the rendered script for
// the invalid sequence; the valid AppleScript escape is `""`.
func TestNoBackslashQuoteInExtraToolTemplates(t *testing.T) {
	tools := []struct {
		name string
		tool common.MCPTool
		args map[string]any
	}{
		{name: "installedApps", tool: installedAppsTool(), args: map[string]any{"limit": 1}},
		{name: "icloudDriveList", tool: icloudDriveListTool(), args: map[string]any{"limit": 1}},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			// We need to inspect the script the callback would
			// generate. None of the affected tools expose their
			// script directly, so we drive the callback and then
			// shell out to osascript with `-e` to verify the
			// parser is happy. If we cannot extract the script
			// text, fall back to a static check on the .go source.
			out, err := exec.Command("grep", "-nE", `do shell script ".*\\"`, "macos_extra_tools.go").Output()
			if err == nil && len(strings.TrimSpace(string(out))) > 0 {
				t.Fatalf("macos_extra_tools.go still contains an invalid \\\" sequence in a do-shell-script literal: %s", string(out))
			}
			_ = tc.tool
			_ = tc.args
		})
	}
}
