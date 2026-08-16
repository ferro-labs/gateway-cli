package table

import "strings"

// A gateway-provided string — a provider name, a plugin summary, an upstream
// error message, a model's answer — is data reaching a terminal, never layout
// and never terminal control. A tab or newline injects a column, a row, or an
// extra line into a pane that owes an exact line count; ESC starts a sequence
// this tool has no business forwarding from an upstream provider to whatever
// is reading stdout. Every surface that renders one of those strings collapses
// them here, so the vocabulary cannot drift between surfaces.
//
// The whole control range goes, not an enumerated handful. Listing the four
// characters that had actually caused a bug (tab, CR, LF, ESC) left BEL ringing
// the terminal and NUL reaching it, and the next escape hatch would have been
// found the same way — by someone hitting it. A terminal reads more than ESC as
// control: C0 and DEL, and C1 0x80–0x9f, where 0x9b is a single-byte CSI that
// opens a sequence exactly as ESC[ does.
func sanitize(s string, keepLF bool) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' && keepLF {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return ' '
		}
		return r
	}, s)
}

// SanitizeCell returns s with every control character collapsed to a single
// space, LF included. For a value that occupies one cell of a table or one
// line of a pane, where a line break would break the caller's line count.
func SanitizeCell(s string) string { return sanitize(s, false) }

// SanitizeText returns s with every control character collapsed to a single
// space except LF, which is kept: for a multi-line body whose line breaks are
// its own (a streamed answer), where the caller splits on \n itself.
func SanitizeText(s string) string { return sanitize(s, true) }
