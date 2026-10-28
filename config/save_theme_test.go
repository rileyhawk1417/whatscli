package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// saveThemeConfig is the config the /theme save fix targets: fourteen
// explicitly overridden color keys plus an explicit theme.
const saveThemeConfig = `[general]
notifications = true

[ui]
chat_sidebar_width = 40

[colors]
theme = dark
background = black
text = "white"
input_background = "#0000ff"
borders = white
`

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "whatscli.config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func readTestConfig(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

// normConfig collapses runs of spaces so asserts survive ini's key
// realignment on save ("theme   = x" becomes "theme = x").
func normConfig(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestSaveThemePresetRemovesOverridesAndBacksUp(t *testing.T) {
	path := writeTestConfig(t, saveThemeConfig)

	if err := SaveTheme(path, "catppuccin", true); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	saved := readTestConfig(t, path)
	if !strings.Contains(normConfig(saved), "theme = catppuccin") {
		t.Errorf("theme key missing after save:\n%s", saved)
	}
	for _, gone := range []string{"background", "input_background", "borders"} {
		if strings.Contains(normConfig(saved), gone+" =") {
			t.Errorf("override key %q survived the save:\n%s", gone, saved)
		}
	}
	if strings.Contains(normConfig(saved), `"#0000ff"`) {
		t.Errorf("quoted hex override survived:\n%s", saved)
	}
	// Unrelated sections survive untouched.
	if strings.Contains(normConfig(saved), "chat_sidebar_width") == false ||
		strings.Contains(normConfig(saved), "notifications") == false ||
		strings.Contains(normConfig(saved), "[general]") == false {
		t.Errorf("[general]/[ui] damaged:\n%s", saved)
	}

	// First write-back leaves a pristine copy.
	backup := readTestConfig(t, path+".bak")
	if backup != saveThemeConfig {
		t.Errorf("backup differs from the original:\n%s", backup)
	}
}

func TestSaveThemeCustomKeepsOverrides(t *testing.T) {
	path := writeTestConfig(t, saveThemeConfig)

	if err := SaveTheme(path, "dark", false); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	saved := readTestConfig(t, path)
	if !strings.Contains(normConfig(saved), "background = black") {
		t.Errorf("custom overrides were deleted:\n%s", saved)
	}
	if !strings.Contains(normConfig(saved), "theme = dark") {
		t.Errorf("theme key missing:\n%s", saved)
	}
	if backup := readTestConfig(t, path+".bak"); backup != saveThemeConfig {
		t.Errorf("backup differs from the original:\n%s", backup)
	}
}

func TestSaveThemeCreatesColorsSection(t *testing.T) {
	path := writeTestConfig(t, "[general]\nnotifications = true\n")

	if err := SaveTheme(path, "mono", false); err != nil {
		t.Fatalf("SaveTheme: %v", err)
	}

	saved := readTestConfig(t, path)
	if !strings.Contains(normConfig(saved), "[colors]") || !strings.Contains(normConfig(saved), "theme = mono") {
		t.Errorf("[colors] section not created:\n%s", saved)
	}
}

func TestSaveThemeRejectsEmptyPath(t *testing.T) {
	if err := SaveTheme("", "dark", false); err == nil {
		t.Error("expected an error for an empty path")
	}
}

// toSnake converts a Colors field name to its INI key per ini.TitleUnderscore
// ("ForwardedText" -> "forwarded_text").
func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// colorNamesMatchStruct asserts colorKeyNames covers exactly the Colors fields
// (minus Theme) so SaveTheme never deletes a key it doesn't know about and
// never leaves a preset-covered key behind.
func TestColorKeyNamesMatchStruct(t *testing.T) {
	// theme itself stays out of the field loop; it is in both sets.
	seen := map[string]bool{"theme": true}
	for i := 0; i < reflect.TypeOf(Colors{}).NumField(); i++ {
		f := reflect.TypeOf(Colors{}).Field(i)
		if f.Anonymous || f.Tag.Get("ini") == "-" {
			continue
		}
		name := toSnake(f.Name)
		seen[name] = true
		if !colorKeyNames[name] {
			t.Errorf("field %q missing from colorKeyNames", name)
		}
	}
	for name := range colorKeyNames {
		if !seen[name] {
			t.Errorf("colorKeyNames has %q, no such Colors field", name)
		}
	}
}

func TestSaveThemeMissingFile(t *testing.T) {
	if err := SaveTheme(filepath.Join(t.TempDir(), "missing.config"), "dark", false); err == nil {
		t.Error("expected an error for a missing config file")
	}
}
