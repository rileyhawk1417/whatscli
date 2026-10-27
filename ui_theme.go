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

	// Derived styles.
	TopBar         lipgloss.Style
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

// timestampColor replaces tcell.ColorDimGray.
const timestampColor = lipgloss.Color("#696969")

// resolveColor accepts the same values the old tcell-based config accepted:
// hex (#rrggbb), ANSI numbers ("4", "213"), or any X11-style name tcell knew
// ("red", "dimgray", "cornflowerblue"). Names are resolved through the
// generated table so existing configs keep working unchanged.
func resolveColor(value string) (lipgloss.Color, error) {
	v := strings.TrimSpace(strings.ToLower(value))
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
// rendering black text.
func loadTheme(c config.Colors) (*theme, error) {
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

	t.TopBar = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Background)
	t.Border = lipgloss.NewStyle().Foreground(t.Borders)
	t.BorderActive = lipgloss.NewStyle().Foreground(t.Borders).Bold(true)
	t.SectionTitle = lipgloss.NewStyle().Foreground(t.ListHeader)
	t.SectionTitleHi = lipgloss.NewStyle().Foreground(t.ListHeader).Bold(true).Reverse(true)
	t.TimeDim = lipgloss.NewStyle().Foreground(timestampColor)
	t.Selected = lipgloss.NewStyle().Reverse(true)
	t.StatusPane = lipgloss.NewStyle().Foreground(statusColor)
	t.Unread = lipgloss.NewStyle().Foreground(t.UnreadCount)
	t.HelpHeading = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	t.HelpKey = lipgloss.NewStyle().Foreground(t.ListHeader).Bold(true)
	t.HelpDim = lipgloss.NewStyle().Foreground(timestampColor)
	t.Error = lipgloss.NewStyle().Foreground(t.Negative)
	t.InputTextStyle = lipgloss.NewStyle().
		Foreground(t.InputText).
		Background(t.InputBackground)
	return t, nil
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
