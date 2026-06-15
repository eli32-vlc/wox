package system

import "testing"

// TestEscapeForAppleScript locks the contract that user-supplied strings are
// always safe to drop inside an AppleScript "…" literal. The helper must
// double double-quotes (so a single `"` survives), double backslashes (so a
// stray `\` does not close a string early on some osascript builds), and
// collapse newlines, tabs, and low C0 control bytes that would otherwise
// break the parser.
func TestEscapeForAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "plain ascii passes through",
			input: "Finder",
			want:  "Finder",
		},
		{
			name:  "single double quote is doubled",
			input: `say "hello"`,
			want:  `say ""hello""`,
		},
		{
			name:  "lone double quote is doubled",
			input: `"`,
			want:  `""`,
		},
		{
			name:  "backslash is doubled",
			input: `a\b`,
			want:  `a\\b`,
		},
		{
			name:  "backslash and quote together",
			input: `path\with"quote`,
			want:  `path\\with""quote`,
		},
		{
			name:  "newline is replaced with space",
			input: "line1\nline2",
			want:  "line1 line2",
		},
		{
			name:  "carriage return is replaced with space",
			input: "line1\rline2",
			want:  "line1 line2",
		},
		{
			name:  "tab is replaced with space",
			input: "col1\tcol2",
			want:  "col1 col2",
		},
		{
			name:  "low control bytes are dropped",
			input: "a\x00b\x01c",
			want:  "abc",
		},
		{
			name:  "unicode survives unchanged",
			input: "résumé — 日本語",
			want:  "résumé — 日本語",
		},
		{
			name:  "embedded null collapses to no char",
			input: "abc\x00def",
			want:  "abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeForAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeForAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
