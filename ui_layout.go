package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// This file holds the pure string-layout primitives used by View(). The old
// implementation delegated all of this to tview's grid; now we draw a single
// outer frame with internal rules, which reproduces tview's shared-border look
// without double lines between panes.

// fit truncates or right-pads s to exactly w display cells. ANSI-aware so
// styled text keeps its escapes intact.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if gap := w - lipgloss.Width(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

// padTo pads s on the right to w cells without truncating.
func padTo(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// frameTop renders "┌────┐".
func frameTop(w int, th *theme) string {
	if w < 2 {
		return strings.Repeat("─", max(w, 0))
	}
	return th.Border.Render("┌" + strings.Repeat("─", w-2) + "┐")
}

// frameBottom renders "└────┘".
func frameBottom(w int, th *theme) string {
	if w < 2 {
		return strings.Repeat("─", max(w, 0))
	}
	return th.Border.Render("└" + strings.Repeat("─", w-2) + "┘")
}

// frameRule renders a horizontal separator spanning the frame, e.g. "├────┤".
func frameRule(w int, th *theme) string {
	if w < 2 {
		return strings.Repeat("─", max(w, 0))
	}
	return th.Border.Render("├" + strings.Repeat("─", w-2) + "┤")
}

// ruleTitled renders a horizontal separator with an inline title:
// "├─ Chats ─────────────┤". The title is highlighted when the section holds
// focus, replacing tview's SetBorderColor-on-focus behaviour.
func ruleTitled(w int, title string, focused bool, th *theme) string {
	if w < 4 {
		return frameRule(w, th)
	}
	style := th.SectionTitle
	if focused {
		style = th.SectionTitleHi
	}
	label := style.Render(" " + title + " ")
	// Leave room for the corners and at least one dash after the label.
	avail := w - 3
	if lipgloss.Width(label) > avail {
		label = ansi.Truncate(label, avail, "…")
	}
	rest := w - 3 - lipgloss.Width(label)
	if rest < 0 {
		rest = 0
	}
	return th.Border.Render("├─") + label + th.Border.Render(strings.Repeat("─", rest)+"┤")
}

// ruleTitledRight renders a separator with a right-aligned title, used for the
// current chat name above the message pane.
func ruleTitledRight(w int, title string, th *theme) string {
	if w < 4 {
		return frameRule(w, th)
	}
	if title == "" {
		return frameRule(w, th)
	}
	label := th.SectionTitle.Render(" " + title + " ")
	avail := w - 4
	if lipgloss.Width(label) > avail {
		label = ansi.Truncate(label, avail, "…")
	}
	dashes := w - 3 - lipgloss.Width(label)
	if dashes < 0 {
		dashes = 0
	}
	return th.Border.Render("├"+strings.Repeat("─", dashes)) + label + th.Border.Render("┤")
}

// vRule is the vertical pane separator, shared between the sidebar and the
// message pane.
func vRule(th *theme) string { return th.Border.Render("│") }

// vRuleFocused renders the separator; it brightens when the message pane holds
// focus so the active side is obvious without border-colour juggling.
func vRuleFocused(focused bool, th *theme) string {
	if focused {
		return th.BorderActive.Render("│")
	}
	return th.Border.Render("│")
}

// frameSide wraps a single content line in the outer vertical borders.
func frameSide(content string, th *theme) string {
	return th.Border.Render("│") + content + th.Border.Render("│")
}

// wrapText wraps styled text to width w, hard-breaking overlong words. Returns
// one entry per display line.
func wrapText(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	wrapped := ansi.Wordwrap(s, w, "")
	wrapped = ansi.Hardwrap(wrapped, w, true)
	return strings.Split(wrapped, "\n")
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clamp returns v limited to [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
