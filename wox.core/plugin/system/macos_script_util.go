package system

import "strings"

// escapeForAppleScript makes a Go string safe to embed inside an AppleScript
// "…" literal. AppleScript uses doubled "" to represent a single inner quote,
// has no raw‑newline string syntax, and a stray backslash can confuse some
// osascript builds. We also strip other C0 control characters so the AI
// cannot smuggle in line terminators or NULL bytes that break the parser.
func escapeForAppleScript(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`""`)
		case '\r', '\n', '\t':
			b.WriteByte(' ')
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
