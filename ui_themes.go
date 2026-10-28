package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/config"
	"gopkg.in/ini.v1"
)

// themePreset is a complete, named palette. Presets exist so users do not have
// to hand-write 14 colour keys to get a readable UI: `theme = dark` selects one
// wholesale (see loadTheme).
//
// Every preset sets its own `background`, because the paint pass covers the
// whole surface (see D10): the terminal's background never shows through, so a
// light preset stays readable on a dark terminal.
type themePreset struct {
	Name   string
	Desc   string
	Colors config.Colors
}

// builtinThemes returns the shipped presets in menu order. Kept in code rather
// than in the config file so a preset never needs 14 keys to be written.
func builtinThemes() []themePreset {
	return []themePreset{
		{
			Name: "dark",
			Desc: "cool tones, dark terminals",
			Colors: config.Colors{
				Background:      "black",
				Text:            "lightgray",
				ForwardedText:   "mediumpurple",
				ListHeader:      "burlywood",
				ListContact:     "darkseagreen",
				ListGroup:       "cornflowerblue",
				ChatContact:     "darkseagreen",
				ChatMe:          "cornflowerblue",
				Borders:         "slategray",
				InputBackground: "darkslategray",
				InputText:       "whitesmoke",
				UnreadCount:     "burlywood",
				Positive:        "darkseagreen",
				Negative:        "lightcoral",
				AutoContrast:    true,
			},
		},
		{
			Name: "light",
			Desc: "for light terminals",
			Colors: config.Colors{
				Background:      "white",
				Text:            "#1f2328",
				ForwardedText:   "#6639ba",
				ListHeader:      "#953800",
				ListContact:     "#116329",
				ListGroup:       "#0550ae",
				ChatContact:     "#116329",
				ChatMe:          "#0550ae",
				Borders:         "#6e7781",
				InputBackground: "#eaeef2",
				InputText:       "#1f2328",
				UnreadCount:     "#953800",
				Positive:        "#116329",
				Negative:        "#a40e26",
				AutoContrast:    true,
			},
		},
		// The three presets below are ports of popular editor colourschemes. Each
		// value is taken from the upstream palette rather than invented, and each
		// foreground is a colour the scheme already uses for that kind of thing:
		//
		//   catppuccin  https://github.com/catppuccin/catppuccin (macchiato)
		//   tokyonight  https://github.com/folke/tokyonight.nvim (storm)
		//   solarized   https://github.com/craftzdog/solarized-osaka.nvim (dark)
		//
		// Roles are mapped by meaning: header/unread take the scheme's warm accent,
		// contacts the green, groups and own messages the blue, borders the faintest
		// tone that still clears the 3:1 graphical floor, and the input row sits on
		// a raised surface. Where a scheme's own choice fell below the text floor
		// (osaka's magenta and red) the brighter step of the same ramp is used, so
		// the preset needs no lifting at load time.
		{
			Name: "catppuccin",
			Desc: "macchiato, soft pastels",
			Colors: config.Colors{
				Background:      "#24273a",
				Text:            "#cad3f5",
				ForwardedText:   "#c6a0f6",
				ListHeader:      "#eed49f",
				ListContact:     "#a6da95",
				ListGroup:       "#8aadf4",
				ChatContact:     "#8bd5ca",
				ChatMe:          "#7dc4e4",
				Borders:         "#6e738d",
				InputBackground: "#494d64",
				InputText:       "#cad3f5",
				UnreadCount:     "#f5a97f",
				Positive:        "#a6da95",
				Negative:        "#ed8796",
				AutoContrast:    true,
			},
		},
		{
			Name: "tokyonight",
			Desc: "storm, deep indigo",
			Colors: config.Colors{
				Background:      "#24283b",
				Text:            "#c0caf5",
				ForwardedText:   "#bb9af7",
				ListHeader:      "#e0af68",
				ListContact:     "#9ece6a",
				ListGroup:       "#7aa2f7",
				ChatContact:     "#73daca",
				ChatMe:          "#2ac3de",
				Borders:         "#737aa2",
				InputBackground: "#292e42",
				InputText:       "#c0caf5",
				UnreadCount:     "#ff9e64",
				Positive:        "#9ece6a",
				Negative:        "#f7768e",
				AutoContrast:    true,
			},
		},
		{
			Name: "solarized",
			Desc: "osaka, sun-baked teal",
			Colors: config.Colors{
				Background:      "#001419",
				Text:            "#adb7b7",
				ForwardedText:   "#8084db",
				ListHeader:      "#f254a1",
				ListContact:     "#849900",
				ListGroup:       "#268bd3",
				ChatContact:     "#29a298",
				ChatMe:          "#268bd3",
				Borders:         "#57767f",
				InputBackground: "#04313f",
				InputText:       "#adb7b7",
				UnreadCount:     "#b28500",
				Positive:        "#849900",
				Negative:        "#f55350",
				AutoContrast:    true,
			},
		},
		{
			Name: "classic",
			Desc: "pre-1.3 ANSI palette",
			Colors: config.Colors{
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
			},
		},
		{
			Name: "mono",
			Desc: "greyscale, no colour",
			Colors: config.Colors{
				Background:      "black",
				Text:            "#c8c8c8",
				ForwardedText:   "#e8e8e8",
				ListHeader:      "#ffffff",
				ListContact:     "#dcdcdc",
				ListGroup:       "#ffffff",
				ChatContact:     "#dcdcdc",
				ChatMe:          "#ffffff",
				Borders:         "#6e6e6e",
				InputBackground: "#303030",
				InputText:       "#ffffff",
				UnreadCount:     "#ffffff",
				Positive:        "#cfcfcf",
				Negative:        "#ffffff",
				AutoContrast:    true,
			},
		},
	}
}

// findTheme looks a preset up by name (case-insensitively).
func findTheme(name string) (themePreset, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range builtinThemes() {
		if p.Name == name {
			return p, true
		}
	}
	// Custom theme files: ~/.config/whatscli/themes/<name>.ini (plan item D2).
	// Built-ins always win over files, so a file cannot shadow a preset.
	if p, ok := loadCustomThemePreset(name); ok {
		return p, true
	}
	return themePreset{}, false
}

// loadCustomThemePreset reads themes/<name>.ini from the config directory and
// builds a palette from its [colors] section. Keys the file does not set fall
// back to the `dark` preset, so a minimal file still renders readably. The
// parsed palette carries no `theme` key of its own, which keeps the lookup
// from recursing. Errors are silent — findTheme only reports "not found", and
// loadThemeWith produces the standard unknown-theme message for a name that
// resolves to nothing.
func loadCustomThemePreset(name string) (themePreset, bool) {
	dir := config.GetThemesDir()
	if dir == "" || name == "" {
		return themePreset{}, false
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return themePreset{}, false
	}
	// IgnoreInlineComment so quoted or bare hex ("#ff79c6") survives the
	// go-ini inline-comment rule that otherwise truncates at "#".
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true},
		filepath.Join(dir, name+".ini"))
	if err != nil {
		return themePreset{}, false
	}
	section, err := cfg.GetSection("colors")
	if err != nil {
		return themePreset{}, false
	}
	colors := builtinThemes()[0].Colors // dark fill for keys the file omits
	cfg.NameMapper = ini.TitleUnderscore
	if err := section.MapTo(&colors); err != nil {
		return themePreset{}, false
	}
	colors.Theme = ""
	return themePreset{Name: name, Desc: "custom (themes/" + name + ".ini)", Colors: colors}, true
}

// customThemeNames lists the custom theme files available in the themes
// directory, sorted, for themeNames and themeChoices.
func customThemeNames() []string {
	dir := config.GetThemesDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ini") {
			continue
		}
		names = append(names, strings.ToLower(strings.TrimSuffix(e.Name(), ".ini")))
	}
	sort.Strings(names)
	return names
}

// themeNames lists the preset names, for error messages.
func themeNames() []string {
	presets := builtinThemes()
	names := make([]string, 0, len(presets))
	for _, p := range presets {
		names = append(names, p.Name)
	}
	return append(names, customThemeNames()...)
}

// customThemeName is the pseudo-preset meaning "use the [colors] values from
// the config file", which is what an empty theme name resolves to.
const customThemeName = "custom"

// themeChoices is the selector menu: the config palette first, then presets.
func themeChoices() []themePreset {
	choices := []themePreset{{
		Name: customThemeName,
		Desc: "your own [colors] values",
	}}
	choices = append(choices, builtinThemes()...)
	for _, name := range customThemeNames() {
		if p, ok := loadCustomThemePreset(name); ok {
			choices = append(choices, p)
		}
	}
	return choices
}

// resolveThemeName maps an empty/current name to the label shown in the UI.
func resolveThemeName(name string) string {
	if name == "" {
		return customThemeName
	}
	return name
}

// ---- selector --------------------------------------------------------------

// themePicker is the /theme overlay: a menu of presets that previews live while
// the cursor moves. It owns no styles so it can be re-rendered after every
// preview in the theme it is previewing.
type themePicker struct {
	choices []themePreset
	cursor  int
	// origin is the preset that was active when the picker opened, restored on
	// Esc/cancel.
	origin string
}

// newThemePicker opens the selector on the currently active theme.
func newThemePicker(current string) *themePicker {
	choices := themeChoices()
	cursor := 0
	for i, c := range choices {
		if c.Name == resolveThemeName(current) {
			cursor = i
			break
		}
	}
	return &themePicker{choices: choices, cursor: cursor, origin: current}
}

// move shifts the cursor and returns the newly highlighted preset name. It
// reports false when the cursor did not move.
func (p *themePicker) move(delta int) (string, bool) {
	next := clamp(p.cursor+delta, 0, len(p.choices)-1)
	if next == p.cursor {
		return "", false
	}
	p.cursor = next
	return p.choices[next].Name, true
}

// selected returns the highlighted preset name.
func (p *themePicker) selected() string { return p.choices[p.cursor].Name }

// render draws the menu as a bordered box sized to its content.
func (p *themePicker) render(th *theme) string {
	const (
		inner = 50
		nameW = 12
	)
	title := lipgloss.NewStyle().Foreground(th.ListHeader).Bold(true)
	name := lipgloss.NewStyle().Foreground(th.Text).Bold(true)
	desc := lipgloss.NewStyle().Foreground(th.Dim)

	lines := make([]string, 0, len(p.choices)+2)
	lines = append(lines, title.Render(padTo(" Themes", inner)))
	for i, c := range p.choices {
		row := fmt.Sprintf(" %s%-*s ", p.marker(i), nameW-1, c.Name)
		text := padTo(row+fit(c.Desc, inner-nameW-4), inner)
		if i == p.cursor {
			lines = append(lines, th.Selected.Render(text))
			continue
		}
		lines = append(lines, name.Render(row)+desc.Render(fit(c.Desc, inner-nameW-4)))
	}
	lines = append(lines, desc.Render(padTo(" ↑/↓ preview   Enter apply   Esc cancel", inner)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Borders).
		Render(strings.Join(lines, "\n"))
}

// marker returns the cursor gutter for a row.
func (p *themePicker) marker(i int) string {
	if i == p.cursor {
		return "▸"
	}
	return " "
}
