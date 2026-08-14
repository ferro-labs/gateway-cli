// Package theme is the ferro console's visual vocabulary: palette, glyph set,
// cell-accurate frames, and the split-F mark.
//
// Every measurement here is in terminal cells (lipgloss.Width), never bytes and
// never runes: CJK is double-width, combining marks are zero-width, and ANSI
// sequences measure zero. A frame padded with len() breaks the first time real
// output arrives.
package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Console palette. Truecolor hex; lipgloss downgrades to the
// terminal's actual profile. Never paint a full-screen background — the
// terminal's own background is the ground.
const (
	colorAccent    = "#d97757"
	colorOK        = "#77ba8d"
	colorWarn      = "#e4b354"
	colorBad       = "#dc6b67"
	colorDim       = "#7f817e"
	colorText      = "#e8e5df"
	colorBright    = "#fff8ef"
	colorFaint     = "#4a4b49"
	colorBorder    = "#3a3b38"
	colorHairline  = "#262726"
	colorSelection = "#1a1717"
)

// minFrameWidth is the narrowest frame that can still close around content:
// two border cells, two padding cells, and something in between.
const minFrameWidth = 8

// Mode is what the terminal will accept. Color is false under NO_COLOR, a
// non-TTY stdout, or TERM=dumb; ASCII is the --ascii glyph downgrade.
type Mode struct {
	Color bool
	ASCII bool
}

// Glyphs is the whole status vocabulary. Color never carries state alone, so
// every state has a glyph here.
type Glyphs struct{ Dot, OK, Warn, Bad, None, Prompt string }

// Glyphs returns the unicode set, or the ASCII set under --ascii.
func (m Mode) Glyphs() Glyphs {
	if m.ASCII {
		return Glyphs{Dot: "[+]", OK: "[OK]", Warn: "[!]", Bad: "[X]", None: "[-]", Prompt: ">"}
	}
	return Glyphs{Dot: "●", OK: "✓", Warn: "!", Bad: "✗", None: "·", Prompt: "❯"}
}

// box is the border vocabulary for one frame style.
type box struct{ tl, tr, bl, br, h, v string }

func (m Mode) box(round bool) box {
	switch {
	case m.ASCII:
		return box{tl: "+", tr: "+", bl: "+", br: "+", h: "-", v: "|"}
	case round:
		return box{tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│"}
	default:
		return box{tl: "┌", tr: "┐", bl: "└", br: "┘", h: "─", v: "│"}
	}
}

// Theme is the rendering surface every TUI file draws through. When
// Mode.Color is false every style is a plain no-op, so callers never branch on
// color themselves.
type Theme struct {
	Mode                                            Mode
	Accent, OK, Warn, Bad, Dim, Text, Bright, Faint lipgloss.Style
	Border, Hairline                                lipgloss.Style
	Selected                                        lipgloss.Style
}

// New builds the theme for a mode. Under !m.Color every field is
// lipgloss.NewStyle(), which renders its input unchanged.
func New(m Mode) Theme {
	fg := func(hex string) lipgloss.Style {
		s := lipgloss.NewStyle()
		if !m.Color {
			return s
		}
		return s.Foreground(lipgloss.Color(hex))
	}
	t := Theme{
		Mode:     m,
		Accent:   fg(colorAccent),
		OK:       fg(colorOK),
		Warn:     fg(colorWarn),
		Bad:      fg(colorBad),
		Dim:      fg(colorDim),
		Text:     fg(colorText),
		Bright:   fg(colorBright),
		Faint:    fg(colorFaint),
		Border:   fg(colorBorder),
		Hairline: fg(colorHairline),
		Selected: lipgloss.NewStyle(),
	}
	if m.Color {
		t.Selected = t.Selected.Background(lipgloss.Color(colorSelection))
	}
	return t
}

// Frame draws a square-corner frame exactly w cells wide, with the title
// embedded in the top border:
//
//	┌─ TITLE ────────┐
//	│ body           │
//	└────────────────┘
//
// Body lines are padded or truncated to fit. An empty title yields an
// unbroken top border. w < minFrameWidth returns the bare body: never emit a
// border that cannot close.
func (t Theme) Frame(title, body string, w int) string {
	return t.frame(title, body, w, t.Mode.box(false))
}

// FrameRound is Frame with rounded corners and no title — the composer and
// modals.
func (t Theme) FrameRound(body string, w int) string {
	return t.frame("", body, w, t.Mode.box(true))
}

func (t Theme) frame(title, body string, w int, b box) string {
	if w < minFrameWidth {
		return body
	}
	lines := strings.Split(body, "\n")
	rows := make([]string, 0, len(lines)+2)
	rows = append(rows, t.top(b, title, w))
	edge := t.Border.Render(b.v)
	for _, line := range lines {
		rows = append(rows, edge+" "+padCell(line, w-4)+" "+edge)
	}
	rows = append(rows, t.Border.Render(b.bl+strings.Repeat(b.h, w-2)+b.br))
	return strings.Join(rows, "\n")
}

func (t Theme) top(b box, title string, w int) string {
	if title == "" {
		return t.Border.Render(b.tl + strings.Repeat(b.h, w-2) + b.tr)
	}
	// Cap the title so at least one trailing rule remains; w >= minFrameWidth
	// leaves at least two cells for it.
	title = ansi.Truncate(title, w-6, "")
	fill := max(w-5-lipgloss.Width(title), 1)
	return t.Border.Render(b.tl+b.h) + " " + t.Dim.Render(title) + " " +
		t.Border.Render(strings.Repeat(b.h, fill)+b.tr)
}

// padCell pads or truncates s to exactly w terminal cells. Truncation is
// ANSI- and grapheme-aware, so styled content keeps its escape sequences
// intact and a clipped double-width rune still leaves the line whole.
func padCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "")
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// markStem is the F's stem: the three rows that carry no arm are the same row.
const markStem = "     ███"

// MarkRows is the split-F, sampled from f_logo.svg — canonical, do not redraw.
var MarkRows = []string{
	"       ████████",
	"     ██████████",
	markStem,
	"       ██████",
	"     ████████",
	markStem,
	markStem,
}

// Mark is the split-F in a rounded panel, drawn two art rows to a text row.
//
// A terminal cell is about twice as tall as it is wide, so the art drawn one
// row per row stands seven lines high — nine inside a panel — beside four
// lines of header text, and reads as a billboard. Half blocks fold it: ▀ inks
// the top half of a cell and ▄ the bottom, so each text row carries two art
// rows and the panel lands six rows over fourteen columns, close enough to
// square on screen. The fold is exact, not a downsample; no art row is dropped
// or merged away.
//
// The panel is what keeps the mark off the screen's corner: without it the art
// starts on the first cell of the first row with nothing around it.
//
// Under --ascii there are no half blocks and no panel — 2h falls below
// minFrameWidth, so FrameRound hands back the letter the mark degrades to.
func Mark(t Theme) string {
	if t.Mode.ASCII {
		return t.Accent.Render("F")
	}
	art := foldedArt()
	widest := 0
	for _, r := range art {
		widest = max(widest, lipgloss.Width(r))
	}
	rows := make([]string, len(art))
	for i, r := range art {
		rows[i] = t.Accent.Render(r)
	}
	// widest + 4: the panel's two border cells and the padding cell FrameRound
	// sets inside each of them.
	return t.FrameRound(strings.Join(rows, "\n"), widest+4)
}

// foldedArt is the canonical rows paired off into half-block text rows.
func foldedArt() []string {
	art := markArt()
	rows := make([]string, 0, (len(art)+1)/2)
	for i := 0; i < len(art); i += 2 {
		bottom := "" // an odd row count leaves the last text row a top half only
		if i+1 < len(art) {
			bottom = art[i+1]
		}
		rows = append(rows, fold(art[i], bottom))
	}
	return rows
}

// fold merges two art rows into one text row of half blocks: a cell is full
// when both rows ink it, a top or bottom half when one does, blank when
// neither.
func fold(topRow, bottomRow string) string {
	top, bottom := []rune(topRow), []rune(bottomRow)
	out := make([]rune, 0, max(len(top), len(bottom)))
	for i := range max(len(top), len(bottom)) {
		inkedTop := i < len(top) && top[i] != ' '
		inkedBottom := i < len(bottom) && bottom[i] != ' '
		switch {
		case inkedTop && inkedBottom:
			out = append(out, '█')
		case inkedTop:
			out = append(out, '▀')
		case inkedBottom:
			out = append(out, '▄')
		default:
			out = append(out, ' ')
		}
	}
	return strings.TrimRight(string(out), " ")
}

// markArt is MarkRows with the shared left margin the SVG sample carried
// measured off, so the mark starts on its own first inked column. The shape is
// untouched: every row loses the same count, so the split arms keep their
// overhang. Byte slicing is safe because the margin is all spaces.
func markArt() []string {
	indent := len(MarkRows[0])
	for _, r := range MarkRows {
		indent = min(indent, len(r)-len(strings.TrimLeft(r, " ")))
	}
	out := make([]string, len(MarkRows))
	for i, r := range MarkRows {
		out[i] = r[indent:]
	}
	return out
}
