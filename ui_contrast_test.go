package main

import (
	"strings"
	"testing"

	"github.com/normen/whatscli/config"
)

// legacyPalette is the [colors] section shipped before the Bubbletea migration
// (and therefore what most existing configs contain).
func legacyPalette() config.Colors {
	return config.Colors{
		Background:      "black",
		Text:            "white",
		ForwardedText:   "purple",
		ListHeader:      "yellow",
		ListContact:     "green",
		ListGroup:       "blue",
		ChatContact:     "green",
		ChatMe:          "blue",
		Borders:         "white",
		InputBackground: "blue",
		InputText:       "white",
		UnreadCount:     "yellow",
		Positive:        "green",
		Negative:        "red",
		AutoContrast:    true,
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		in   string
		want rgb
		ok   bool
	}{
		{"#fff", rgb{255, 255, 255}, true},
		{"#7aa2f7", rgb{0x7a, 0xa2, 0xf7}, true},
		{"#7AA2F7", rgb{0x7a, 0xa2, 0xf7}, true},
		{"#0000", rgb{}, false},
		{"#ggg", rgb{}, false},
		{"0", rgb{0, 0, 0}, true},
		{"4", rgb{0, 0, 128}, true},
		{"15", rgb{255, 255, 255}, true},
		{"196", rgb{255, 0, 0}, true},
		{"244", rgb{128, 128, 128}, true},
		{"255", rgb{238, 238, 238}, true},
		{"256", rgb{}, false},
		{"default", rgb{}, false},
	}
	for _, tt := range tests {
		got, ok := parseColor(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("parseColor(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestContrastRatio(t *testing.T) {
	black, white := rgb{0, 0, 0}, rgb{255, 255, 255}
	if got := contrastRatio(black, white); got < 20.9 || got > 21.1 {
		t.Errorf("black/white ratio = %.2f, want 21", got)
	}
	if got := contrastRatio(black, black); got != 1 {
		t.Errorf("identical colors ratio = %.2f, want 1", got)
	}
}

func TestEnsureContrast(t *testing.T) {
	black := rgb{0, 0, 0}

	t.Run("compliant color untouched", func(t *testing.T) {
		white := rgb{255, 255, 255}
		got, changed := ensureContrast(white, black, minContrast)
		if changed || got != white {
			t.Errorf("white on black should be left alone, got %v changed=%v", got, changed)
		}
	})

	t.Run("ansi blue is raised, hue kept", func(t *testing.T) {
		blue := rgb{0, 0, 255} // 2.4:1 on black
		got, changed := ensureContrast(blue, black, minContrast)
		if !changed {
			t.Fatal("expected blue on black to be adjusted")
		}
		if r := contrastRatio(got, black); r < minContrast {
			t.Errorf("adjusted ratio %.2f, want >= %.1f", r, minContrast)
		}
		if !(got.b > got.r && got.b > got.g) {
			t.Errorf("hue not preserved: %v", got)
		}
	})

	t.Run("light background darkens", func(t *testing.T) {
		white, gray := rgb{255, 255, 255}, rgb{0x77, 0x77, 0x77}
		got, changed := ensureContrast(gray, white, minContrast)
		if !changed {
			t.Fatal("expected gray on white to be adjusted")
		}
		if r := contrastRatio(got, white); r < minContrast {
			t.Errorf("adjusted ratio %.2f on light background, want >= %.1f", r, minContrast)
		}
		if got.relativeLuminance() > gray.relativeLuminance() {
			t.Errorf("color moved the wrong way: %v", got)
		}
	})
}

func TestDimColorIsReadable(t *testing.T) {
	black := rgb{0, 0, 0}
	text := rgb{0xc8, 0xd0, 0xda}
	dim := dimColor(text, black)
	if r := contrastRatio(dim, black); r < minContrast {
		t.Errorf("dim color ratio %.2f, want >= %.1f", r, minContrast)
	}
	if dim.relativeLuminance() >= text.relativeLuminance() {
		t.Error("dim color should be darker than the text color")
	}
}

func TestLoadThemeRaisesLegacyPalette(t *testing.T) {
	th, err := loadTheme(legacyPalette())
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	black, _ := parseColor(string(th.Background))
	for name, col := range map[string]string{
		"text":           string(th.Text),
		"forwarded_text": string(th.ForwardedText),
		"list_header":    string(th.ListHeader),
		"list_contact":   string(th.ListContact),
		"list_group":     string(th.ListGroup),
		"chat_contact":   string(th.ChatContact),
		"chat_me":        string(th.ChatMe),
		"unread_count":   string(th.UnreadCount),
		"positive":       string(th.Positive),
		"negative":       string(th.Negative),
	} {
		c, ok := parseColor(col)
		if !ok {
			t.Fatalf("%s: unparsable color %q", name, col)
		}
		if r := contrastRatio(c, black); r < minContrast {
			t.Errorf("%s = %s has %.2f:1 contrast, want >= %.1f", name, col, r, minContrast)
		}
	}

	// The dark tones the old config used must be the ones that moved.
	if string(th.ChatMe) == "#0000ff" {
		t.Error("chat_me (blue) should have been lightened")
	}
	if string(th.ForwardedText) == "#800080" {
		t.Error("forwarded_text (purple) should have been lightened")
	}
	// White text was fine and must stay exactly white.
	if string(th.Text) != "#ffffff" {
		t.Errorf("text should stay #ffffff, got %s", th.Text)
	}
	// Blue input field keeps its background; only the text is measured on it.
	if string(th.InputBackground) != "#0000ff" {
		t.Errorf("input_background should stay #0000ff, got %s", th.InputBackground)
	}

	notice := th.contrastNotice()
	for _, want := range []string{"chat_me", "forwarded_text", "auto_contrast"} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice %q missing %q", notice, want)
		}
	}
	if strings.Contains(notice, "text #ffffff") {
		t.Error("compliant colors should not be reported")
	}
}

func TestLoadThemeAutoContrastOff(t *testing.T) {
	c := legacyPalette()
	c.AutoContrast = false
	th, err := loadTheme(c)
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	if string(th.ChatMe) != "#0000ff" {
		t.Errorf("chat_me = %s, want exact configured #0000ff", th.ChatMe)
	}
	if string(th.ForwardedText) != "#800080" {
		t.Errorf("forwarded_text = %s, want exact configured #800080", th.ForwardedText)
	}
	if len(th.Adjusted) != 0 {
		t.Errorf("expected no adjustments, got %v", th.Adjusted)
	}
	if th.contrastNotice() != "" {
		t.Error("expected no notice when auto_contrast is off")
	}
}

func TestLoadThemeDefaultsAreReadable(t *testing.T) {
	th, err := loadTheme(*config.Config.Colors)
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	if len(th.Adjusted) != 0 {
		t.Errorf("shipped defaults should not need adjustment, got %v", th.Adjusted)
	}
}

func TestLoadThemeDerivesDim(t *testing.T) {
	th, err := loadTheme(legacyPalette())
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	dim, ok := parseColor(string(th.Dim))
	if !ok {
		t.Fatalf("dim is not a concrete color: %q", th.Dim)
	}
	black, _ := parseColor(string(th.Background))
	if r := contrastRatio(dim, black); r < minContrast {
		t.Errorf("dim %.2f:1, want >= %.1f", r, minContrast)
	}
}

func TestResolveColorAcceptsQuoting(t *testing.T) {
	// go-ini stores hex values quoted (a bare "#" would start a comment), and
	// users hand-editing the file tend to quote them too.
	for _, in := range []string{`"#C8D0DA"`, "`#c8d0da`", "'#c8d0da'", " #c8d0da "} {
		got, err := resolveColor(in)
		if err != nil {
			t.Errorf("resolveColor(%q): %v", in, err)
			continue
		}
		if string(got) != "#c8d0da" {
			t.Errorf("resolveColor(%q) = %q, want #c8d0da", in, got)
		}
	}
	if _, err := resolveColor("``"); err == nil {
		t.Error("expected an error for an empty quoted value")
	}
}
