package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/config"
)

func TestPaintLineRearmsBackgroundAfterResets(t *testing.T) {
	withTrueColor(t)
	const seq = "\x1b[48;2;255;0;0m"

	// A styled span must not drop back to the terminal background after its
	// own reset — that reset is where palettes used to "leak".
	out := paintLine("\x1b[38;2;0;0;255mname\x1b[0m rest", 20, seq)
	plain := ansiPlain(out)
	if plain != "name rest"+"           " {
		t.Fatalf("text or padding changed: %q", plain)
	}
	if strings.Count(out, seq) != 3 {
		t.Fatalf("expected the sequence on the prefix, after the reset and on the padding: %q", out)
	}
	if !strings.HasSuffix(out, ansiReset) {
		t.Fatal("painted padding must end in a reset")
	}
}

func TestPaintLineCutAndNoop(t *testing.T) {
	withTrueColor(t)
	if out := paintLine("abcdefghij", 5, "\x1b[48;2;255;0;0m"); ansiPlain(out) != "abcde" {
		t.Fatalf("overlong line was not cut: %q", ansiPlain(out))
	}
	// Without a sequence the line is only padded with plain spaces, exactly the
	// pre-painting behaviour.
	if out := paintLine("ab", 5, ""); ansiPlain(out) != "ab   " {
		t.Fatalf("unexpected padding: %q", ansiPlain(out))
	}
	// Negative width only re-arms the background.
	if out := paintLine("\x1b[31mx\x1b[0m", -1, "\x1b[48;5;1m"); !strings.Contains(out, "48;5;1m") {
		t.Fatalf("negative width did not re-arm: %q", out)
	}
}

func TestThemeDerivesBackgroundSequences(t *testing.T) {
	withTrueColor(t)
	th, err := loadTheme(*config.Config.Colors)
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	// black background on the dark palette.
	if !strings.Contains(th.backgroundSeq, "48;2;0;0;0m") && !strings.Contains(th.backgroundSeq, "48;5;") {
		t.Fatalf("backgroundSeq = %q, want a black background SGR", th.backgroundSeq)
	}
	// darkslategray input field.
	if !strings.Contains(th.inputSeq, "48;2;47;79;79") {
		t.Fatalf("inputSeq = %q, want darkslategray", th.inputSeq)
	}
	if !strings.HasPrefix(th.backgroundSeq, "\x1b[") || strings.Contains(th.backgroundSeq, " ") {
		t.Fatalf("backgroundSeq is not a bare SGR prefix: %q", th.backgroundSeq)
	}
}

func TestViewPaintsWholeSurface(t *testing.T) {
	withTrueColor(t)
	m := themeModel(t)
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) != m.height {
		t.Fatalf("got %d lines, want %d", len(lines), m.height)
	}
	th := m.th
	for i, line := range lines {
		if w := lipgloss.Width(line); w != m.width {
			t.Fatalf("line %d is %d cells wide, want %d", i, w, m.width)
		}
		want := th.backgroundSeq
		if i == len(lines)-2 { // input row
			want = th.inputSeq
		}
		if want != "" && !strings.Contains(line, want) {
			t.Fatalf("line %d is not painted with %q: %q", i, want, ansiPlain(line))
		}
	}
}

func TestViewPaintedOnColorlessProfile(t *testing.T) {
	// Tests run without a TTY, so the profile cannot map colors: the paint pass
	// degrades to plain padding and must not mangle the frame.
	m := themeModel(t)
	out := m.View()
	if strings.Contains(out, "48;2;") {
		t.Fatalf("colorless profile painted a background: %q", out[:120])
	}
	if !strings.Contains(out, "WhatsCLI") {
		t.Fatalf("frame content lost: %q", out[:120])
	}
}

func TestThemePickerOverlayIsPainted(t *testing.T) {
	withTrueColor(t)
	m := themeModel(t)
	m.picker = newThemePicker("dark")
	out := m.View()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		seq := m.th.backgroundSeq
		if i == len(lines)-2 { // input row
			seq = m.th.inputSeq
		}
		if !strings.Contains(line, seq) {
			t.Fatalf("overlay left line %d unpainted: %q", i, ansiPlain(line))
		}
	}
	// The picker still renders on top.
	if !strings.Contains(ansiPlain(out), "Themes") {
		t.Fatal("picker box missing from the painted view")
	}
}
