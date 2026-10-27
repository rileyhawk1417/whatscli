package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Terminals only repaint cells the program prints, so "drawing the background"
// means carrying the theme's background color on every cell of the frame: the
// sequence is re-armed after every style reset, short lines are padded with
// painted spaces, and overlong lines are cut.
const ansiReset = "\x1b[0m"

// paintLine makes line exactly width cells wide on the given background. seq is
// a background SGR prefix from the theme; an empty seq paints nothing and keeps
// the terminal's own background, which is what happens on color-less profiles.
// A width below 0 pads nothing and only re-arms the background, used for boxes
// whose rows are padded again by the caller.
func paintLine(line string, width int, seq string) string {
	if seq != "" {
		// Arm the surface from the first cell — lines of plain text carry no
		// escape at all — and re-arm it after every style reset that follows.
		// Themes style text with per-segment lipgloss styles, and foreign ANSI
		// blocks (QR codes, images) carry their own color spans; every one of
		// them ends in a reset that would drop back to the terminal background.
		line = seq + strings.ReplaceAll(line, ansiReset, ansiReset+seq)
	}
	if width < 0 {
		return line
	}
	w := lipgloss.Width(line)
	if w > width {
		return ansi.Cut(line, 0, width)
	}
	if pad := width - w; pad > 0 {
		if seq == "" {
			return line + strings.Repeat(" ", pad)
		}
		return line + seq + strings.Repeat(" ", pad) + ansiReset
	}
	return line
}