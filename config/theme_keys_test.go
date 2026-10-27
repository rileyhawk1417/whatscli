package config

import (
	"testing"

	"gopkg.in/ini.v1"
)

func TestColorKeysListsWhatTheFileContains(t *testing.T) {
	cfg, err := ini.LoadSources(ini.LoadOptions{}, []byte("[colors]\ntheme = dark\nchat_me = red\nborders = \"#334455\"\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	section, err := cfg.GetSection("colors")
	if err != nil {
		t.Fatalf("section: %v", err)
	}

	keys := colorKeys(section)
	for _, want := range []string{"theme", "chat_me", "borders"} {
		if !keys[want] {
			t.Errorf("expected %q to be reported as present", want)
		}
	}
	if keys["text"] {
		t.Error("text was not in the file but is reported as present")
	}
}

func TestColorKeysIsEmptyWithoutSection(t *testing.T) {
	if keys := colorKeys(&ini.Section{}); len(keys) != 0 {
		t.Fatalf("expected no keys, got %v", keys)
	}
}

func TestDefaultThemeIsDark(t *testing.T) {
	// The default preset must match the default palette. It is safe to name it
	// because presets only fill in keys the config file does not set (see
	// loadThemeWith in the UI package).
	if Config.Colors.Theme != "dark" {
		t.Errorf("expected default theme dark, got %q", Config.Colors.Theme)
	}
}
