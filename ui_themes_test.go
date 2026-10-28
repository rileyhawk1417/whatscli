package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

// withTrueColor forces a color-capable profile for assertions on escape
// sequences; the default renderer strips color when stdout is not a TTY.
func withTrueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

// themeModel builds a model with a real, resolved theme.
func themeModel(t *testing.T) uiModel {
	t.Helper()
	th, err := loadThemeWith(*config.Config.Colors, nil)
	if err != nil {
		t.Fatalf("loadTheme: %v", err)
	}
	sm := &messages.SessionManager{CommandChannel: make(chan messages.Command, 16)}
	m := newUIModel(th, sm, keyMap{global: map[string]action{}, messages: map[string]action{}})
	m.resize(100, 30)
	return m
}

// themeFields lists every palette entry with its name, mirroring the ordering
// loadTheme uses, so tests can walk the whole palette.
func themeFields(th *theme) map[string]lipgloss.Color {
	return map[string]lipgloss.Color{
		"text":             th.Text,
		"forwarded_text":   th.ForwardedText,
		"list_header":      th.ListHeader,
		"list_contact":     th.ListContact,
		"list_group":       th.ListGroup,
		"chat_contact":     th.ChatContact,
		"chat_me":          th.ChatMe,
		"borders":          th.Borders,
		"unread_count":     th.UnreadCount,
		"positive":         th.Positive,
		"negative":         th.Negative,
		"input_text":       th.InputText,
		"background":       th.Background,
		"input_background": th.InputBackground,
	}
}

func TestBuiltinThemesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range builtinThemes() {
		if p.Name == "" || p.Desc == "" {
			t.Fatalf("preset %+v is missing a name or description", p)
		}
		if p.Name != strings.ToLower(p.Name) || strings.ContainsAny(p.Name, " \t") {
			t.Errorf("preset name %q must be a lowercase word", p.Name)
		}
		if seen[p.Name] {
			t.Errorf("duplicate preset name %q", p.Name)
		}
		seen[p.Name] = true

		th, err := loadTheme(p.Colors)
		if err != nil {
			t.Fatalf("preset %q does not load: %v", p.Name, err)
		}
		for field, col := range themeFields(th) {
			if string(col) == "" {
				t.Errorf("preset %q: %s is empty", p.Name, field)
			}
		}
		// The shipped palettes must not need touching up: only "classic"
		// deliberately reuses the dark ANSI tones that auto_contrast lifts.
		if p.Name != "classic" && len(th.Adjusted) > 0 {
			t.Errorf("preset %q needed contrast fixes: %v", p.Name, th.Adjusted)
		}
	}
}

// TestEditorPresetsMatchUpstreamBases pins the ports in theme-refs/ to the
// background and body text of the scheme they were taken from, so a future
// palette refresh is a deliberate change rather than a silent drift.
func TestEditorPresetsMatchUpstreamBases(t *testing.T) {
	for _, tc := range []struct{ name, background, text string }{
		{"catppuccin", "#24273a", "#cad3f5"}, // Catppuccin Macchiato base/text
		{"tokyonight", "#24283b", "#c0caf5"}, // Tokyo Night Storm bg/fg
		{"solarized", "#001419", "#adb7b7"},  // Solarized Osaka bg/base1
	} {
		preset, ok := findTheme(tc.name)
		if !ok {
			t.Fatalf("preset %q is missing", tc.name)
		}
		if preset.Colors.Background != tc.background {
			t.Errorf("%s background = %q, want %q", tc.name, preset.Colors.Background, tc.background)
		}
		if preset.Colors.Text != tc.text {
			t.Errorf("%s text = %q, want %q", tc.name, preset.Colors.Text, tc.text)
		}
	}
}

// TestPickerFitsEveryPresetName guards the widened name column: a name wider
// than the column would push the description left and break the box frame.
// TestFullyOverriddenConfigStillSeesPresets reproduces the reported bug: a
// config that sets every [colors] key made every preset render identical to
// the startup palette, because the pick merged the overrides back in.
func TestFullyOverriddenConfigStillSeesPresets(t *testing.T) {
	m := themeModel(t)
	m.baseColors = *config.Config.Colors
	m.baseColors.Background = "black"
	m.baseColors.Text = "white"
	prev := config.ColorKeysSet
	config.ColorKeysSet = map[string]bool{}
	for _, k := range []string{"background", "text", "forwarded_text", "list_header", "list_contact",
		"list_group", "chat_contact", "chat_me", "borders", "input_background", "input_text",
		"unread_count", "positive", "negative"} {
		config.ColorKeysSet[k] = true
	}
	t.Cleanup(func() { config.ColorKeysSet = prev })

	if err := m.applyThemeName("catppuccin"); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}
	wantBg, _ := resolveColor("#24273a")
	wantText, _ := resolveColor("#cad3f5")
	if m.th.Background != wantBg {
		t.Errorf("catppuccin pick kept the config background %q", m.th.Background)
	}
	if m.th.Text != wantText {
		t.Errorf("catppuccin pick kept the config text %q", m.th.Text)
	}

	// The picker cursor starts on catppuccin in this state, and restoring
	// "custom" must bring the original palette back.
	p := newThemePicker(m.themeName)
	if p.selected() != "catppuccin" {
		t.Errorf("picker opened on %q, want the active preset", p.selected())
	}
	if err := m.applyThemeName(""); err != nil {
		t.Fatalf("applyThemeName(custom): %v", err)
	}
	black, _ := resolveColor("black")
	white, _ := resolveColor("white")
	if m.th.Background != black || m.th.Text != white {
		t.Error("custom did not restore the startup palette")
	}
}

func TestPickerFitsEveryPresetName(t *testing.T) {
	for _, p := range themeChoices() {
		if len(p.Name) > 11 {
			t.Errorf("preset %q is %d cells wide; widen themePicker.render's nameW", p.Name, len(p.Name))
		}
	}
}

func TestFindThemeIsCaseInsensitive(t *testing.T) {
	for _, name := range []string{"dark", "DARK", " Dark "} {
		if _, ok := findTheme(name); !ok {
			t.Errorf("findTheme(%q) failed", name)
		}
	}
	if _, ok := findTheme("nope"); ok {
		t.Error("findTheme accepted an unknown name")
	}
	if _, ok := findTheme(""); ok {
		t.Error("findTheme accepted the empty name")
	}
}

func TestPresetColorsClearContrastFloor(t *testing.T) {
	for _, p := range builtinThemes() {
		th, err := loadTheme(p.Colors)
		if err != nil {
			t.Fatalf("preset %q: %v", p.Name, err)
		}
		backdrop, ok := parseColor(string(th.Background))
		if !ok {
			t.Fatalf("preset %q: unparsable background %q", p.Name, th.Background)
		}
		inputBackdrop, ok := parseColor(string(th.InputBackground))
		if !ok {
			inputBackdrop = backdrop
		}
		for field, col := range themeFields(th) {
			if field == "background" || field == "input_background" {
				continue
			}
			fg, ok := parseColor(string(col))
			if !ok {
				t.Fatalf("preset %q: unparsable %s %q", p.Name, field, col)
			}
			surface := backdrop
			floor := minContrast
			if field == "input_text" {
				surface = inputBackdrop
			}
			if field == "borders" {
				floor = borderContrast
			}
			if got := contrastRatio(fg, surface); got < floor {
				t.Errorf("preset %q: %s has contrast %.2f, want >= %.1f", p.Name, field, got, floor)
			}
		}
	}
}

func TestThemeKeyAppliesPreset(t *testing.T) {
	dark, _ := findTheme("dark")
	base := *config.Config.Colors
	base.Theme = "dark"
	base.ChatMe = "red"

	// No overrides: the preset wins wholesale (what the selector does).
	plain, err := loadThemeWith(base, nil)
	if err != nil {
		t.Fatalf("loadThemeWith: %v", err)
	}
	if want := dark.Colors.ChatMe; string(plain.ChatMe) != resolveHex(t, want) {
		t.Errorf("chat_me = %s, want the preset value %s", plain.ChatMe, want)
	}

	// With the key recorded as present, the user's own value wins and the rest
	// of the preset stays.
	overridden, err := loadThemeWith(base, map[string]bool{"chat_me": true})
	if err != nil {
		t.Fatalf("loadThemeWith: %v", err)
	}
	if string(overridden.ChatMe) == string(plain.ChatMe) {
		t.Error("chat_me override was ignored")
	}
	if string(overridden.Text) != string(plain.Text) {
		t.Errorf("text changed even though it was not overridden: %s vs %s", overridden.Text, plain.Text)
	}
}

func TestEmptyColorValueFallsBackToPreset(t *testing.T) {
	// go-ini turns `text = #ff0000` into an empty value (the "#" starts a
	// comment), so the preset must survive instead of failing startup.
	base := *config.Config.Colors
	base.Theme = "dark"
	base.Text = ""

	th, err := loadThemeWith(base, map[string]bool{"text": true})
	if err != nil {
		t.Fatalf("loadThemeWith: %v", err)
	}
	if th.Text == "" {
		t.Fatal("text is empty; the preset entry was not kept")
	}
}

func TestUnknownThemeMentionsAvailableNames(t *testing.T) {
	base := *config.Config.Colors
	base.Theme = "neon"
	_, err := loadThemeWith(base, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
	for _, want := range append([]string{"colors.theme", "neon"}, themeNames()...) {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestThemeChoicesStartWithCustom(t *testing.T) {
	choices := themeChoices()
	if len(choices) != len(builtinThemes())+1 {
		t.Fatalf("got %d choices, want %d", len(choices), len(builtinThemes())+1)
	}
	if choices[0].Name != customThemeName {
		t.Fatalf("first choice is %q, want %q", choices[0].Name, customThemeName)
	}
	if got := resolveThemeName(""); got != customThemeName {
		t.Errorf("resolveThemeName(\"\") = %q", got)
	}
	if got := resolveThemeName("light"); got != "light" {
		t.Errorf("resolveThemeName(light) = %q", got)
	}
}

func TestThemePickerStartsOnCurrentAndClamps(t *testing.T) {
	p := newThemePicker("light")
	if p.selected() != "light" {
		t.Fatalf("cursor started on %q, want light", p.selected())
	}
	if name, moved := p.move(-100); !moved || name != customThemeName {
		t.Errorf("move(-100) = %q, %v; want custom, true", name, moved)
	}
	if name, moved := p.move(100); !moved || name != "mono" {
		t.Errorf("move(100) = %q, %v; want the last preset, true", name, moved)
	}
	if _, moved := p.move(1); moved {
		t.Error("move past the end reported a move")
	}
	if p.origin != "light" {
		t.Errorf("origin = %q, want light", p.origin)
	}
}

func TestThemeCommandAppliesPreset(t *testing.T) {
	m := themeModel(t)
	m.input.SetValue("/theme light")
	updated, cmd := m.submitInput()
	m = updated.(uiModel)
	if cmd != nil {
		t.Error("UI-side theme command should not talk to the backend")
	}
	if m.themeName != "light" {
		t.Fatalf("themeName = %q, want light", m.themeName)
	}
	if string(m.th.Background) != "#ffffff" {
		t.Errorf("background = %s, want white", m.th.Background)
	}
	if !strings.Contains(m.msgs.blocks[len(m.msgs.blocks)-1].text, "theme: light") {
		t.Errorf("no confirmation in the message pane: %+v", m.msgs.blocks[len(m.msgs.blocks)-1])
	}
}

func TestThemeCommandRejectsUnknownName(t *testing.T) {
	m := themeModel(t)
	m.input.SetValue("/theme neon")
	updated, _ := m.submitInput()
	m = updated.(uiModel)
	if m.themeName != "dark" {
		t.Errorf("themeName = %q, want the unchanged default", m.themeName)
	}
	if last := m.msgs.blocks[len(m.msgs.blocks)-1].text; !strings.Contains(last, "unknown theme") {
		t.Errorf("expected an error line, got %q", last)
	}
}

func TestThemePickerPreviewsAndRestoresOnEsc(t *testing.T) {
	m := themeModel(t)
	// A distinctive custom background so preview changes are observable.
	m.baseColors = *config.Config.Colors
	m.baseColors.Theme = ""
	m.baseColors.Background = "white"
	if err := m.applyThemeName(""); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}

	m.input.SetValue("/theme")
	updated, _ := m.submitInput()
	m = updated.(uiModel)
	if m.picker == nil {
		t.Fatal("/theme did not open the selector")
	}
	if got := m.picker.selected(); got != customThemeName {
		t.Fatalf("selector opened on %q, want %q", got, customThemeName)
	}
	if got := string(m.th.Background); got != "#ffffff" {
		t.Fatalf("background = %s, want the custom palette", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // → dark
	m = updated.(uiModel)
	if got := string(m.th.Background); got != "#000000" {
		t.Errorf("preview did not apply the next theme, background = %s", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(uiModel)
	if m.picker != nil {
		t.Error("Esc left the selector open")
	}
	if got := string(m.th.Background); got != "#ffffff" {
		t.Errorf("Esc did not restore the previous palette, background = %s", got)
	}
	if m.themeName != "" {
		t.Errorf("themeName = %q after cancel, want empty", m.themeName)
	}
}

func TestThemePickerEnterKeepsSelection(t *testing.T) {
	m := themeModel(t)
	m.input.SetValue("/theme")
	updated, _ := m.submitInput()
	m = updated.(uiModel)

	// The selector opens on the active preset, so one step down from "dark"
	// lands on "light".
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(uiModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(uiModel)

	if m.picker != nil {
		t.Error("Enter left the selector open")
	}
	if m.themeName != "light" {
		t.Errorf("themeName = %q, want light", m.themeName)
	}
	if got := string(m.th.Background); got != "#ffffff" {
		t.Errorf("background = %s after apply", got)
	}
}

func TestThemePickerSwallowsPaneKeys(t *testing.T) {
	m := themeModel(t)
	m.input.SetValue("/theme")
	updated, _ := m.submitInput()
	m = updated.(uiModel)

	// "q" closes the selector instead of typing into the input.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(uiModel)
	if m.picker != nil {
		t.Error("q did not close the selector")
	}
	if m.input.Value() != "" {
		t.Errorf("input = %q, want empty", m.input.Value())
	}
}

func TestThemeChangeRetintsMessageHistory(t *testing.T) {
	withTrueColor(t)

	m := themeModel(t)
	updated, _ := m.Update(msgNewMessage{msg: messages.Message{Id: "1", ContactName: "Ada", Text: "hello", Timestamp: 1600000000}})
	m = updated.(uiModel)
	before := m.msgs.blocks[0].styled
	if before == "" {
		t.Fatal("message block was not rendered")
	}

	if err := m.applyThemeName("light"); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}
	after := m.msgs.blocks[0].styled
	if after == before {
		t.Fatal("history kept the old colors after the theme change")
	}
	if ansiPlain(after) != ansiPlain(before) {
		t.Errorf("message text changed: %q vs %q", ansiPlain(after), ansiPlain(before))
	}
	if !strings.Contains(after, "38;2;") {
		t.Errorf("expected a truecolor escape in the re-rendered message: %q", after)
	}
	// The free-form notice keeps its text too, and the pane still has content.
	if len(m.msgs.blocks) != 1 || len(m.msgs.lines) == 0 {
		t.Errorf("retint dropped content: %d blocks, %d lines", len(m.msgs.blocks), len(m.msgs.lines))
	}
}

func TestRetintLeavesANSIPassthroughAlone(t *testing.T) {
	m := themeModel(t)
	qr := "\x1b[47m  \x1b[0m\x1b[40m  \x1b[0m"
	m.msgs.addStyled(qr, false)
	if err := m.applyThemeName("mono"); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}
	if len(m.msgs.blocks) != 1 || m.msgs.blocks[0].text != qr {
		t.Fatalf("passthrough block was rewritten: %+v", m.msgs.blocks)
	}
}

func TestOverlayCenterPreservesFrame(t *testing.T) {
	base := strings.Join([]string{
		"aaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccc",
		"dddddddddddddddddddd",
		"eeeeeeeeeeeeeeeeeeee",
	}, "\n")
	box := "┌──┐\n│xy│\n└──┘"

	out := overlayCenter(base, box, 20, 5)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 20 {
			t.Fatalf("line %d is %d cells wide, want 20: %q", i, w, line)
		}
	}
	if !strings.Contains(lines[2], "│xy│") {
		t.Errorf("box is not on the middle row: %q", lines[2])
	}
	if !strings.HasPrefix(ansiPlain(lines[2]), "ccccccc") {
		t.Errorf("frame behind the box was overwritten: %q", ansiPlain(lines[2]))
	}
	if overlayCenter(base, "", 20, 5) != base {
		t.Error("empty box changed the frame")
	}
}

// resolveHex resolves a configured color to the value the theme would hold.
func resolveHex(t *testing.T, value string) string {
	t.Helper()
	col, err := resolveColor(value)
	if err != nil {
		t.Fatalf("resolveColor(%q): %v", value, err)
	}
	return string(col)
}

// ansiPlain strips escape sequences for text comparisons.
func ansiPlain(s string) string { return ansi.Strip(s) }

func TestPresetPickReplacesPaletteWholesale(t *testing.T) {
	m := themeModel(t)
	m.baseColors = *config.Config.Colors
	m.baseColors.ChatMe = "red"
	prev := config.ColorKeysSet
	config.ColorKeysSet = map[string]bool{"chat_me": true}
	t.Cleanup(func() { config.ColorKeysSet = prev })

	preset, _ := findTheme("mono")
	monoChat, err := resolveColor(preset.Colors.ChatMe)
	if err != nil {
		t.Fatalf("resolveColor: %v", err)
	}

	if err := m.applyThemeName("mono"); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}
	if m.th.ChatMe != monoChat {
		t.Error("picking a preset did not replace chat_me wholesale")
	}
	if m.th.Text != presetColorsText(t, "mono") {
		t.Error("picking a preset did not apply the rest of the preset")
	}

	// "custom" returns to exactly what startup produced, overrides included.
	if err := m.applyThemeName(""); err != nil {
		t.Fatalf("applyThemeName: %v", err)
	}
	red, _ := resolveColor("red")
	if m.th.ChatMe != red {
		t.Errorf("custom did not restore the chat_me override (got %q)", m.th.ChatMe)
	}
	if m.th.Text == presetColorsText(t, "mono") {
		t.Error("custom still renders the mono preset")
	}
}

// presetColorsText resolves a preset's text color without contrast adjustments.
func presetColorsText(t *testing.T, name string) lipgloss.Color {
	t.Helper()
	p, ok := findTheme(name)
	if !ok {
		t.Fatalf("unknown preset %q", name)
	}
	col, err := resolveColor(p.Colors.Text)
	if err != nil {
		t.Fatalf("resolveColor: %v", err)
	}
	return col
}
