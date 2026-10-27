package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/config"
)

// theme holds every color resolved from the [colors] INI section plus the
// lipgloss styles built from them. It is constructed once at startup so render
// paths never touch config or parse color names.
type theme struct {
	// Raw palette (also used for markup in message text).
	Background      lipgloss.Color
	Text            lipgloss.Color
	ForwardedText   lipgloss.Color
	ListHeader      lipgloss.Color
	ListContact     lipgloss.Color
	ListGroup       lipgloss.Color
	ChatContact     lipgloss.Color
	ChatMe          lipgloss.Color
	Borders         lipgloss.Color
	InputBackground lipgloss.Color
	InputText       lipgloss.Color
	UnreadCount     lipgloss.Color
	Positive        lipgloss.Color
	Negative        lipgloss.Color
	// Dim is derived, not configured: the text color pulled towards the
	// background for timestamps and secondary hints.
	Dim lipgloss.Color

	// Adjusted lists the palette entries whose lightness had to be raised to
	// stay readable (empty when the configured colors already passed).
	Adjusted []string

	// Derived styles.
	Border         lipgloss.Style
	BorderActive   lipgloss.Style
	SectionTitle   lipgloss.Style
	SectionTitleHi lipgloss.Style
	TimeDim        lipgloss.Style
	Selected       lipgloss.Style
	StatusPane     lipgloss.Style
	Unread         lipgloss.Style
	HelpHeading    lipgloss.Style
	HelpKey        lipgloss.Style
	HelpDim        lipgloss.Style
	Error          lipgloss.Style
	InputTextStyle lipgloss.Style
}

// statusColor is the fixed accent used for the broadcast/status pane, matching
// the previous tcell.ColorYellow usage in RenderChatTable.
const statusColor = lipgloss.Color("#ffff00")

// borderContrast is the WCAG AA ratio for graphical objects: borders are a
// frame, not body text, so they may stay quieter than the palette.
const borderContrast = 3.0

// resolveColor accepts the same values the old tcell-based config accepted:
// hex (#rrggbb), ANSI numbers ("4", "213"), or any X11-style name tcell knew
// ("red", "dimgray", "cornflowerblue"). Names are resolved through the
// generated table so existing configs keep working unchanged.
func resolveColor(value string) (lipgloss.Color, error) {
	// go-ini cannot store a bare "#rrggbb" (it starts an inline comment), so hex
	// values are written quoted; strip the quoting here for convenience.
	v := strings.Trim(strings.TrimSpace(value), "`\"'")
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "", fmt.Errorf("empty color value")
	}
	if strings.HasPrefix(v, "#") {
		if len(v) != 7 && len(v) != 4 {
			return "", fmt.Errorf("invalid hex color %q", value)
		}
		return lipgloss.Color(v), nil
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 || n > 255 {
			return "", fmt.Errorf("color index %d out of range (0-255)", n)
		}
		return lipgloss.Color(v), nil
	}
	if hex, ok := colorNameHex[v]; ok {
		return lipgloss.Color(hex), nil
	}
	return "", fmt.Errorf("unknown color %q", value)
}

// loadTheme resolves the whole [colors] section, failing on the first invalid
// entry so misconfiguration is reported at startup instead of silently
// rendering black text. Foreground colors are lifted until they are legible on
// the configured background (see ui_contrast.go) unless auto_contrast is off.
func loadTheme(c config.Colors) (*theme, error) { return loadThemeWith(c, nil) }

// loadThemeWith resolves a palette, optionally on top of a named preset.
// overrides names the color keys the config file actually contains, so
// `theme = dark` plus `chat_me = red` keeps the preset for everything except
// chat_me. A nil overrides map applies the given palette wholesale.
func loadThemeWith(c config.Colors, overrides map[string]bool) (*theme, error) {
	name := strings.ToLower(strings.TrimSpace(c.Theme))
	if name != "" {
		preset, ok := findTheme(name)
		if !ok {
			return nil, fmt.Errorf("colors.theme: unknown theme %q (available: %s)",
				c.Theme, strings.Join(themeNames(), ", "))
		}
		c = mergePalette(preset.Colors, c, overrides)
	}

	type pair struct {
		name  string
		value string
		dst   *lipgloss.Color
	}
	t := &theme{}
	pairs := []pair{
		{"background", c.Background, &t.Background},
		{"text", c.Text, &t.Text},
		{"forwarded_text", c.ForwardedText, &t.ForwardedText},
		{"list_header", c.ListHeader, &t.ListHeader},
		{"list_contact", c.ListContact, &t.ListContact},
		{"list_group", c.ListGroup, &t.ListGroup},
		{"chat_contact", c.ChatContact, &t.ChatContact},
		{"chat_me", c.ChatMe, &t.ChatMe},
		{"borders", c.Borders, &t.Borders},
		{"input_background", c.InputBackground, &t.InputBackground},
		{"input_text", c.InputText, &t.InputText},
		{"unread_count", c.UnreadCount, &t.UnreadCount},
		{"positive", c.Positive, &t.Positive},
		{"negative", c.Negative, &t.Negative},
	}
	for _, p := range pairs {
		col, err := resolveColor(p.value)
		if err != nil {
			return nil, fmt.Errorf("colors.%s: %w", p.name, err)
		}
		*p.dst = col
	}

	background, ok := parseColor(string(t.Background))
	if !ok {
		background = rgb{0, 0, 0} // unknown but valid value (e.g. "default")
	}

	if c.AutoContrast {
		t.applyContrast(background)
	}

	t.TimeDim = lipgloss.NewStyle().Foreground(t.Dim)
	t.HelpDim = lipgloss.NewStyle().Foreground(t.Dim)

	t.Border = lipgloss.NewStyle().Foreground(t.Borders)
	t.BorderActive = lipgloss.NewStyle().Foreground(t.Borders).Bold(true)
	t.SectionTitle = lipgloss.NewStyle().Foreground(t.ListHeader)
	t.SectionTitleHi = lipgloss.NewStyle().Foreground(t.ListHeader).Bold(true).Reverse(true)
	t.Selected = lipgloss.NewStyle().Reverse(true)
	t.StatusPane = lipgloss.NewStyle().Foreground(statusColor)
	t.Unread = lipgloss.NewStyle().Foreground(t.UnreadCount)
	t.HelpHeading = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	t.HelpKey = lipgloss.NewStyle().Foreground(t.ListHeader).Bold(true)
	t.Error = lipgloss.NewStyle().Foreground(t.Negative)
	t.InputTextStyle = lipgloss.NewStyle().
		Foreground(t.InputText).
		Background(t.InputBackground)
	return t, nil
}

// mergePalette layers the user's own color keys over a preset. AutoContrast is
// always the user's choice: a preset never forces adjustments on or off.
func mergePalette(base, user config.Colors, overrides map[string]bool) config.Colors {
	base.AutoContrast = user.AutoContrast
	if len(overrides) == 0 {
		return base
	}
	set := func(key, value string, dst *string) {
		// An empty value means the key was present but unparsable (e.g. an
		// unquoted hex that go-ini read as a comment); keep the preset entry
		// rather than failing startup.
		if overrides[key] && strings.TrimSpace(value) != "" {
			*dst = value
		}
	}
	set("background", user.Background, &base.Background)
	set("text", user.Text, &base.Text)
	set("forwarded_text", user.ForwardedText, &base.ForwardedText)
	set("list_header", user.ListHeader, &base.ListHeader)
	set("list_contact", user.ListContact, &base.ListContact)
	set("list_group", user.ListGroup, &base.ListGroup)
	set("chat_contact", user.ChatContact, &base.ChatContact)
	set("chat_me", user.ChatMe, &base.ChatMe)
	set("borders", user.Borders, &base.Borders)
	set("input_background", user.InputBackground, &base.InputBackground)
	set("input_text", user.InputText, &base.InputText)
	set("unread_count", user.UnreadCount, &base.UnreadCount)
	set("positive", user.Positive, &base.Positive)
	set("negative", user.Negative, &base.Negative)
	return base
}

// applyContrast raises every foreground color to a readable ratio against the
// surface it is drawn on. The input text is measured against the input field
// background, everything else against the main background; backgrounds
// themselves are left alone.
func (t *theme) applyContrast(background rgb) {
	inputBackground, ok := parseColor(string(t.InputBackground))
	if !ok {
		inputBackground = background
	}

	entries := []struct {
		name     string
		dst      *lipgloss.Color
		backdrop rgb
		min      float64
	}{
		{"text", &t.Text, background, minContrast},
		{"forwarded_text", &t.ForwardedText, background, minContrast},
		{"list_header", &t.ListHeader, background, minContrast},
		{"list_contact", &t.ListContact, background, minContrast},
		{"list_group", &t.ListGroup, background, minContrast},
		{"chat_contact", &t.ChatContact, background, minContrast},
		{"chat_me", &t.ChatMe, background, minContrast},
		{"borders", &t.Borders, background, borderContrast},
		{"input_text", &t.InputText, inputBackground, minContrast},
		{"unread_count", &t.UnreadCount, background, minContrast},
		{"positive", &t.Positive, background, minContrast},
		{"negative", &t.Negative, background, minContrast},
	}
	for _, e := range entries {
		raw := string(*e.dst)
		from, ok := parseColor(raw)
		if !ok {
			continue // not something we can reason about (e.g. "default")
		}
		adjusted, changed := ensureContrast(from, e.backdrop, e.min)
		if !changed {
			continue
		}
		*e.dst = lipgloss.Color(adjusted.hex())
		t.Adjusted = append(t.Adjusted, contrastNote(e.name, from, adjusted, e.backdrop))
	}

	text, ok := parseColor(string(t.Text))
	if !ok {
		text = rgb{255, 255, 255}
	}
	t.Dim = lipgloss.Color(dimColor(text, background).hex())
}

// contrastNotice is the one-line startup hint shown when the palette had to be
// touched up, or "" when it did not.
func (t *theme) contrastNotice() string {
	if len(t.Adjusted) == 0 {
		return ""
	}
	return "Raised for readability: " + strings.Join(t.Adjusted, ", ") +
		"\nSet [colors] auto_contrast = false to keep the exact values."
}

// colorListLines renders the color-name preview, replacing the tcell.ColorNames
// iteration the old ShowColorList used.
func (t *theme) colorListLines() []string {
	lines := make([]string, 0, len(colorNamesSorted))
	for _, name := range colorNamesSorted {
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(colorNameHex[name]))
		lines = append(lines, swatch.Render(name))
	}
	return lines
}
