package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/config"
)

// themePreset is a complete, named palette. Presets exist so users do not have
// to hand-write 14 colour keys to get a readable UI: `theme = dark` selects one
// wholesale (see loadTheme).
//
// Every preset assumes the terminal background matches its name and leaves
// `background` at "black" or "white" accordingly, because the UI only paints
// the top bar and the input line — the rest of the frame keeps the terminal's
// own background.
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
	return themePreset{}, false
}

// themeNames lists the preset names, for error messages.
func themeNames() []string {
	presets := builtinThemes()
	names := make([]string, 0, len(presets))
	for _, p := range presets {
		names = append(names, p.Name)
	}
	return names
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
	return append(choices, builtinThemes()...)
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
		inner = 46
		nameW = 10
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
