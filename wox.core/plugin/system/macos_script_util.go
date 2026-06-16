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

// appleScriptStringLiteral returns an AppleScript expression that evaluates
// to the value of s. It can be used anywhere a literal is expected —
// including inside `whose` filter clauses — because embedded double quotes
// are spliced with the `quote` predefined constant instead of relying on the
// `""` string-escape (which is only valid inside `"…"` literals and is
// rejected by the parser inside a `whose` filter).
//
// The result is one of two shapes:
//
//   - No embedded quotes:  "literal text"
//   - With embedded quotes: "before " & quote & "middle" & quote & "after"
//
// Backslashes are still doubled so the literal round-trips through any
// shell-string context, and C0 control bytes are dropped to keep the
// script parseable.
func appleScriptStringLiteral(s string) string {
	if s == "" {
		return `""`
	}
	// First sanitize: drop C0 control bytes and collapse newlines/tabs
	// to spaces. This mirrors escapeForAppleScript's behavior so the
	// output is safe to drop into any AppleScript string context.
	var clean strings.Builder
	clean.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\r', '\n', '\t':
			clean.WriteByte(' ')
		default:
			if r < 0x20 {
				continue
			}
			clean.WriteRune(r)
		}
	}
	cleaned := clean.String()
	if !strings.ContainsRune(cleaned, '"') && !strings.ContainsRune(cleaned, '\\') {
		// Fast path: no special characters — a plain literal is enough.
		return `"` + cleaned + `"`
	}
	// Split on `"` so each segment can be its own literal, then
	// rejoin the segments with `& quote &` between them. Also double
	// any backslashes so they survive inside the new literals.
	segments := strings.Split(cleaned, `"`)
	var b strings.Builder
	for i, seg := range segments {
		if i > 0 {
			b.WriteString(` & quote & `)
		}
		b.WriteByte('"')
		// Double backslashes inside the segment so the parser sees
		// them as literal characters in the new "…" literal.
		b.WriteString(strings.ReplaceAll(seg, `\`, `\\`))
		b.WriteByte('"')
	}
	return b.String()
}
