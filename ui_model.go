package main

import (
	"fmt"
	"github.com/skratchdot/open-golang/open"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

// focusArea identifies which pane owns keyboard input. It replaces tview's
// implicit focus tracking (ctx.App.GetFocus()).
type focusArea int

const (
	focusStatus focusArea = iota
	focusChats
	focusGroups
	focusMessages
	focusInput
)

func (f focusArea) String() string {
	switch f {
	case focusStatus:
		return "status"
	case focusChats:
		return "chats"
	case focusGroups:
		return "groups"
	case focusMessages:
		return "messages"
	case focusInput:
		return "input"
	}
	return "unknown"
}

// uiModel is the whole UI state. The backend talks to it exclusively through
// the message types declared below, delivered by uiBridge (see ui_handler.go).
type uiModel struct {
	th *theme

	width, height int
	ready         bool

	side  sidebar
	msgs  messagesPane
	input textinput.Model
	keys  keyMap

	focus        focusArea
	statusInfo   messages.SessionStatus
	currentChat  messages.Chat
	mouseEnabled bool

	// Palette bookkeeping. baseColors is the [colors] section as loaded from
	// the config file; themeName is the preset currently in effect ("" = the
	// config palette). The /theme selector only changes these in memory.
	baseColors config.Colors
	themeName  string
	picker     *themePicker

	// Login screen bookkeeping. showQR runs once a second (countdown tick),
	// so it records what it last rendered: the whole screen is rebuilt only
	// when the QR payload/attempt changes, and a countdown tick updates just
	// the header line. See the post-milestone fix note in
	// doc/bubbletea-migration/00-design-decisions.md.
	qrRendered  string // QR payload currently on screen ("" = none yet)
	qrHeaderIdx int    // block index of the countdown header, only valid after first render

	sm   *messages.SessionManager
	prog *tea.Program
}

// newUIModel wires the initial model state. The program pointer is attached
// later (see main.go) because the backend handler needs it during construction.
func newUIModel(th *theme, sm *messages.SessionManager, keys keyMap) uiModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = ""
	ti.CharLimit = 0
	ti.SetValue("")
	ti.Focus()
	ti.PromptStyle = th.InputTextStyle
	ti.TextStyle = th.InputTextStyle
	ti.Cursor.Style = th.InputTextStyle.Reverse(true)
	ti.Cursor.TextStyle = th.InputTextStyle

	return uiModel{
		th:         th,
		side:       newSidebar(batchSize),
		msgs:       newMessagesPane(th),
		input:      ti,
		keys:       keys,
		focus:      focusInput,
		sm:         sm,
		baseColors: *config.Config.Colors,
		themeName:  strings.ToLower(strings.TrimSpace(config.Config.Colors.Theme)),
		statusInfo: messages.SessionStatus{
			Connected: false,
		},
	}
}

// Init implements tea.Model.
func (m uiModel) Init() tea.Cmd { return textinput.Blink }

// ---- backend messages ------------------------------------------------------

type (
	msgNewMessage struct{ msg messages.Message }
	msgScreen     struct{ msgs []messages.Message }
	msgChatList   struct{ pq []*messages.Conversation }
	msgSession    struct{ status messages.SessionStatus }
	msgText       struct{ text string }
	msgError      struct{ err error }
	msgStyled     struct {
		text string
		wrap bool
	}
	msgUpdateQR struct {
		qr      string
		attempt int
		timeout int
	}
	msgClear    struct{}
	msgQuit     struct{}
	msgOpenFile struct{ path string }
	// msgHelp and msgColorList are rendered in the model rather than in the
	// backend handler, so they follow a runtime theme change.
	msgHelp      struct{}
	msgColorList struct{}
	msgImageText struct {
		text string
		err  error
	}
)

// ---- Update ----------------------------------------------------------------

// Update implements tea.Model. All state transitions happen here; there is no
// concurrent mutation of UI state anymore, unlike the QueueUpdateDraw model.
func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case msgNewMessage:
		m.msgs.addMessage(msg.msg, m.th)
		return m, nil

	case msgScreen:
		m.msgs.setScreen(msg.msgs, m.th)
		if len(msg.msgs) == 0 {
			if m.currentChat.Id == "" {
				m.msgs.addText(helpText(m.th), m.th)
			} else {
				m.msgs.addText(emptyChatText(m.th), m.th)
			}
		}
		m.msgs.vp.GotoBottom()
		return m, nil

	case msgChatList:
		m.side.setConversations(msg.pq)
		return m, nil

	case msgSession:
		m.statusInfo = msg.status
		return m, nil

	case msgText:
		m.msgs.addText(msg.text, m.th)
		return m, nil

	case msgError:
		m.msgs.addText(errorText(m.th, msg.err), m.th)
		return m, nil

	case msgStyled:
		m.msgs.addStyled(msg.text, msg.wrap)
		return m, nil

	case msgUpdateQR:
		m.showQR(msg.qr, msg.attempt, msg.timeout)
		return m, nil

	case msgClear:
		m.msgs.clear()
		m.msgs.addText(helpText(m.th), m.th)
		return m, nil

	case msgHelp:
		m.msgs.addStyled(helpText(m.th), true)
		return m, nil

	case msgColorList:
		m.msgs.addStyled(strings.Join(m.th.colorListLines(), "\n"), true)
		return m, nil

	case msgQuit:
		return m, tea.Quit

	case msgImageText:
		if msg.err != nil {
			m.msgs.addText(errorText(m.th, msg.err), m.th)
		}
		if msg.text != "" {
			m.msgs.addStyled(msg.text, false)
		}
		return m, nil

	case msgOpenFile:
		// Fire-and-forget external viewer, same as UiHandler.OpenFile did.
		return m, openFileCmd(msg.path)
	}

	// Everything else (blink ticks, etc.) goes to the focused input.
	if m.focus == focusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// resize recomputes the pane geometry. Geometry is derived from the terminal
// size and the configured sidebar width, so only this method needs to know the
// layout constants.
func (m *uiModel) resize(w, h int) {
	if w <= 0 || h <= 0 {
		// Some terminals report no size on startup; fall back to a usable default
		// instead of showing the placeholder forever.
		w, h = 80, 24
	}
	m.width, m.height = w, h
	m.ready = w >= 24 && h >= 10
	if !m.ready {
		return
	}
	innerW := w - 2
	innerH := h - 2

	sbW := clamp(config.Config.Ui.ChatSidebarWidth, 12, max(12, innerW-20))
	msgW := innerW - sbW - 1
	mainH := innerH - 4 // top bar + title rule + rule above input + input line

	// The sidebar spans the message rows plus the rule and input rows, matching
	// the old grid where the table filled both grid rows.
	m.side.width, m.side.height = sbW, mainH+2
	m.msgs.setSize(msgW, mainH)
	m.input.Width = max(1, msgW-1) // bubbles pads the view one cell past Width
}

// ---- key handling ----------------------------------------------------------

// handleKey dispatches keystrokes. Config bindings win over widget handling,
// which matches the old tview app-level input capture order. The theme selector,
// when open, takes every key so nothing leaks to the panes behind it.
func (m uiModel) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.picker != nil {
		return m.handlePickerKey(k)
	}

	key := k.String()

	if a := m.keys.lookup(key, m.focus); a != actNone {
		return m.applyAction(a)
	}

	switch m.focus {
	case focusInput:
		switch key {
		case "enter":
			return m.submitInput()
		case "esc":
			m.input.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd

	case focusMessages:
		switch key {
		case "up", "k":
			m.msgs.moveSelection(-1)
		case "down", "j":
			m.msgs.moveSelection(1)
		case "pgup", "ctrl+u":
			m.msgs.moveSelection(-10)
		case "pgdown", "ctrl+d":
			m.msgs.moveSelection(10)
		case "g", "home":
			m.msgs.selectFirst()
		case "G", "end":
			m.msgs.selectLast()
		case "esc":
			m.msgs.clearSelection()
		case "tab":
			m.msgs.clearSelection()
			m.focus = focusInput
		}
		return m, nil

	default: // sidebar panes
		switch key {
		case "up", "ctrl+u":
			m.moveSidebar(-1)
		case "down", "ctrl+d":
			m.moveSidebar(1)
		case "tab":
			m.cycleFocus()
		case "enter":
			m.openSelectedChat()
		}
		return m, nil
	}
}

// handleMouse handles clicks and wheel scrolling. Mouse reporting is only active
// after the user presses Ctrl+p (decision D4), so terminal text selection keeps
// working by default.
func (m uiModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.picker != nil {
		return m, nil // the selector is keyboard-only so clicks cannot hit the panes
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.msgs.vp.ScrollUp(3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.msgs.vp.ScrollDown(3)
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	return m.handleClick(msg.X, msg.Y)
}

// handleClick maps terminal coordinates onto the panes: frame row 0, top bar 1,
// chat-title rule 2, content rows 3..3+mainH-1, rule, input line, bottom frame.
func (m uiModel) handleClick(x, y int) (tea.Model, tea.Cmd) {
	mainH := m.side.height - 2
	if y < 3 || y >= 3+mainH {
		return m, nil
	}
	row := y - 3

	// Sidebar: select the clicked conversation and open it.
	if x >= 1 && x < 1+m.side.width {
		pane, idx, ok := m.side.hitTest(row)
		if !ok || idx < 0 || idx >= len(m.side.rows(pane)) {
			return m, nil
		}
		m.focus = pane
		m.side.setSel(pane, idx)
		m.openSelectedChat()
		return m, nil
	}

	// Message pane: select the clicked message.
	if x > 1+m.side.width && x < m.width-1 {
		line := m.msgs.vp.YOffset + row
		if msgIdx := m.msgs.entryAtLine(line); msgIdx >= 0 {
			m.focus = focusMessages
			m.msgs.selectEntry(msgIdx)
		}
	}
	return m, nil
}

// applyAction executes a config-bound action.
func (m uiModel) applyAction(a action) (tea.Model, tea.Cmd) {
	switch a {
	case actFocusMessages:
		m.focus = focusMessages
		if len(m.msgs.entries) > 0 && m.msgs.selected < 0 {
			m.msgs.selectLast()
		}

	case actFocusInput:
		m.msgs.clearSelection()
		m.focus = focusInput

	case actFocusChats:
		m.msgs.clearSelection()
		m.focus = focusChats

	case actSwitchPanels:
		m.cycleFocus()

	case actCommandRead:
		return m, m.sendCommand("read", nil)

	case actCommandBacklog:
		return m, m.sendCommand("backlog", nil)

	case actCommandConnect:
		return m, m.sendCommand("login", nil)

	case actCommandHelp:
		m.msgs.addText(helpText(m.th), m.th)

	case actCommandQuit:
		// Send the disconnect before quitting so the backend can shut down cleanly.
		sm := m.sm
		return m, func() tea.Msg {
			sm.CommandChannel <- messages.Command{Name: "disconnect"}
			return tea.Quit()
		}

	case actToggleMouse:
		m.mouseEnabled = !m.mouseEnabled
		if m.mouseEnabled {
			m.msgs.addText("Mouse interaction ENABLED (App selection enabled)", m.th)
			return m, enableMouseCmd()
		}
		m.msgs.addText("Mouse interaction DISABLED (Native selection enabled)", m.th)
		return m, disableMouseCmd()

	case actCopyUser:
		m.copyUserID()

	case actPasteUser:
		text, err := clipboard.ReadAll()
		if err != nil {
			m.msgs.addText("failed to paste: "+err.Error(), m.th)
			return m, nil
		}
		m.input.SetValue(m.input.Value() + " " + text)
		m.input.CursorEnd()

	case actMessageDownload, actMessageOpen, actMessageShow, actMessageUrl, actMessageInfo, actMessageRevoke:
		if msg := m.msgs.selectedMessage(); msg != nil {
			name := messageActionName(a)
			m.msgs.clearSelection()
			m.focus = focusInput
			return m, m.sendCommand(name, []string{msg.Id})
		}
	}
	return m, nil
}

// messageActionName maps a message action to its backend command name, which is
// the same mapping the old handleMessageCommand closure held.
func messageActionName(a action) string {
	switch a {
	case actMessageDownload:
		return "download"
	case actMessageOpen:
		return "open"
	case actMessageShow:
		return "show"
	case actMessageUrl:
		return "url"
	case actMessageInfo:
		return "info"
	case actMessageRevoke:
		return "revoke"
	}
	return ""
}

// copyUserID ports handleCopyUser: the selected message's contact wins, then the
// current chat.
func (m *uiModel) copyUserID() {
	if msg := m.msgs.selectedMessage(); msg != nil {
		if err := clipboard.WriteAll(msg.ContactId); err != nil {
			m.msgs.addText("failed to copy: "+err.Error(), m.th)
		} else {
			name := msg.ContactName
			if name == "" {
				name = msg.ContactShort
			}
			m.msgs.addText("copied id of "+name+" to clipboard", m.th)
		}
		m.msgs.clearSelection()
		return
	}
	if m.currentChat.Id != "" {
		if err := clipboard.WriteAll(m.currentChat.Id); err != nil {
			m.msgs.addText("failed to copy: "+err.Error(), m.th)
		} else {
			m.msgs.addText("copied id of "+m.currentChat.Name+" to clipboard", m.th)
		}
	}
}

// submitInput ports EnterCommand.
func (m uiModel) submitInput() (tea.Model, tea.Cmd) {
	value := m.input.Value()
	if value == "" {
		return m, nil
	}
	m.input.SetValue("")

	prefix := config.Config.General.CmdPrefix
	if strings.HasPrefix(value, prefix) {
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return m, nil
		}
		name := strings.TrimPrefix(fields[0], prefix)
		var params []string
		if len(fields) > 1 {
			params = fields[1:]
		}
		// The theme lives in the UI model, not the backend registry.
		if name == "theme" || name == "themes" {
			return m.handleThemeCommand(params), nil
		}
		return m, m.sendCommand(name, params)
	}

	if m.currentChat.Id == "" {
		m.msgs.addText("no receiver", m.th)
		return m, nil
	}
	return m, m.sendCommand("send", []string{m.currentChat.Id, value})
}

// ---- themes ----------------------------------------------------------------

// handleThemeCommand handles /theme. With no argument it opens the selector;
// with one it applies that preset directly.
func (m uiModel) handleThemeCommand(params []string) uiModel {
	if len(params) == 0 {
		m.picker = newThemePicker(m.themeName)
		return m
	}
	return m.selectTheme(params[0])
}

// selectTheme applies a preset and reports the result in the message pane.
func (m uiModel) selectTheme(name string) uiModel {
	if err := m.applyThemeName(name); err != nil {
		m.msgs.addText(errorText(m.th, err), m.th)
		return m
	}
	label := resolveThemeName(m.themeName)
	line := "theme: " + label
	if m.themeName != "" && m.themeName != strings.ToLower(strings.TrimSpace(m.baseColors.Theme)) {
		line += fmt.Sprintf(" (set [colors] theme = %s to keep it)", m.themeName)
	}
	m.msgs.addText(line, m.th)
	return m
}

// previewTheme applies a preset for the selector preview and swallows errors.
func (m *uiModel) previewTheme(name string) {
	if err := m.applyThemeName(name); err != nil {
		m.msgs.addText(errorText(m.th, err), m.th)
	}
}

// handlePickerKey drives the /theme overlay: arrows preview live, Enter keeps,
// Esc restores whatever was active when the selector opened.
func (m uiModel) handlePickerKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.picker
	switch k.String() {
	case "up", "k", "ctrl+u", "shift+tab":
		if name, moved := p.move(-1); moved {
			m.previewTheme(name)
		}
	case "down", "j", "ctrl+d", "tab":
		if name, moved := p.move(1); moved {
			m.previewTheme(name)
		}
	case "home", "g":
		if name, moved := p.move(-len(p.choices)); moved {
			m.previewTheme(name)
		}
	case "end", "G":
		if name, moved := p.move(len(p.choices)); moved {
			m.previewTheme(name)
		}
	case "enter":
		name := p.selected()
		m.picker = nil
		return m.selectTheme(name), nil
	case "esc", "q", "ctrl+c":
		origin := p.origin
		m.picker = nil
		m.previewTheme(origin)
	}
	return m, nil
}

// applyThemeName switches the whole UI to a preset; the empty name restores the
// palette from the config file. Nothing is written to disk.
func (m *uiModel) applyThemeName(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == customThemeName {
		name = ""
	}

	var (
		th  *theme
		err error
	)
	if name == "" {
		// Rebuild exactly what startup produced, including any color keys that
		// fine-tune the configured preset.
		th, err = loadThemeWith(m.baseColors, config.ColorKeysSet)
	} else {
		preset, ok := findTheme(name)
		if !ok {
			return fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(themeNames(), ", "))
		}
		palette := mergePalette(preset.Colors, m.baseColors, config.ColorKeysSet)
		palette.Theme = "" // the palette now holds the preset values
		th, err = loadThemeWith(palette, nil)
	}
	if err != nil {
		return err
	}

	m.themeName = name
	m.th = th
	m.retint()
	return nil
}

// retint re-renders everything that baked a color into a string.
func (m *uiModel) retint() {
	m.msgs.retint(m.th)
	ti := m.input
	ti.PromptStyle = m.th.InputTextStyle
	ti.TextStyle = m.th.InputTextStyle
	ti.Cursor.Style = m.th.InputTextStyle.Reverse(true)
	ti.Cursor.TextStyle = m.th.InputTextStyle
	m.input = ti
}

// ---- sidebar navigation ----------------------------------------------------

// cycleFocus implements SwitchPanels / Tab: status → chats → groups → messages
// → input → status (skipping the status pane when it is empty).
func (m *uiModel) cycleFocus() {
	m.msgs.clearSelection()
	switch m.focus {
	case focusInput, focusMessages:
		m.focus = m.firstSidebarPane()
	case focusStatus:
		m.focus = focusChats
	case focusChats:
		m.focus = focusGroups
	case focusGroups:
		m.focus = focusMessages
	}
}

// firstSidebarPane returns the status pane when it has rows, otherwise chats.
func (m *uiModel) firstSidebarPane() focusArea {
	if len(m.side.status) > 0 {
		return focusStatus
	}
	return focusChats
}

// moveSidebar moves within the focused pane, jumping to the neighbouring pane at
// the edges — the behaviour the four handle*PanelUp/Down functions implemented.
func (m *uiModel) moveSidebar(delta int) {
	pane := m.focus
	rows := m.side.rows(pane)
	if len(rows) == 0 {
		m.jumpSidebar(delta)
		return
	}
	sel := m.side.selOf(pane)

	if delta < 0 && sel == 0 {
		m.jumpSidebar(-1)
		return
	}
	if delta > 0 && sel == len(rows)-1 {
		if pane == focusChats && m.side.limit < len(m.side.all) {
			// Grow the infinite-scroll window before leaving the pane.
			m.side.moveCursor(pane, 1)
			m.openSelectedChat()
			return
		}
		m.jumpSidebar(1)
		return
	}

	if m.side.moveCursor(pane, delta) {
		m.openSelectedChat()
	}
}

// jumpSidebar moves focus to the next/previous sidebar pane at a boundary.
func (m *uiModel) jumpSidebar(delta int) {
	switch m.focus {
	case focusStatus:
		if delta > 0 {
			m.focus = focusChats
			m.side.selectFirst(focusChats)
		}
	case focusChats:
		if delta > 0 {
			m.focus = focusGroups
			m.side.selectFirst(focusGroups)
		} else {
			if len(m.side.status) > 0 {
				m.focus = focusStatus
				m.side.selectFirst(focusStatus)
			}
		}
	case focusGroups:
		if delta < 0 {
			m.focus = focusChats
			m.side.selectLast(focusChats)
		}
	}
	m.openSelectedChat()
}

// openSelectedChat loads the conversation under the cursor (port of
// SetDisplayedChat plus the openFrom* callbacks).
func (m *uiModel) openSelectedChat() {
	conv := m.side.selectedConversation(m.focus)
	if conv == nil {
		return
	}
	chat := messages.Chat{
		Id:          conv.JID,
		Name:        conv.Name,
		Unread:      int(conv.Unread),
		LastMessage: conv.LastMsgTime,
		IsGroup:     strings.HasSuffix(conv.JID, messages.GROUPSUFFIX),
	}
	if conv.Name == "" {
		chat.Name = conv.JID
	}
	m.currentChat = chat
	m.msgs.clear()
	m.msgs.setTitle(chat.Name)
	// The reply arrives asynchronously as msgScreen.
	m.queueCommand("select", []string{chat.Id})
}

// queueCommand hands a command to the backend without ever blocking the UI
// event loop on a full CommandChannel.
func (m *uiModel) queueCommand(name string, params []string) {
	cmd := messages.Command{Name: name, Params: params}
	select {
	case m.sm.CommandChannel <- cmd:
	default:
		sm := m.sm
		go func() { sm.CommandChannel <- cmd }()
	}
}

// ---- QR --------------------------------------------------------------------

// showQR updates the login screen: help text, header (with live countdown),
// and the QR block. It is a port of UiHandler.UpdateQR, with two behaviour
// fixes that restore the pre-migration semantics — see the post-milestone
// fix note in doc/bubbletea-migration/00-design-decisions.md:
//
//   - The input line and focus are never touched. The old handler rewrote only
//     the text area, so typing survived each one-second countdown tick; the
//     port used to clear the input and force focus back every tick, which made
//     the login screen untypeable.
//   - The whole screen is rebuilt only when the QR payload or its attempt
//     number changes. A same-QR countdown tick updates just the countdown
//     line, so help/error output printed beside the QR is not wiped every
//     second, and the viewport is not yanked to the bottom.
func (m *uiModel) showQR(qr string, attempt, timeout int) {
	header := ""
	if timeout > 0 {
		header = fmt.Sprintf("\n\n=== QR Code Generated (Attempt %d) - Auto-refreshing in %ds ===\n\nScan this with WhatsApp:\n\n", attempt, timeout)
	} else {
		header = fmt.Sprintf("\n\n=== QR Code Generated (Attempt %d) - Awaiting new QR code from WhatsApp Server ===\n\nScan this with WhatsApp:\n\n", attempt)
	}

	if qr != m.qrRendered {
		// New QR code (or first render): rebuild the login screen, exactly like
		// the old handler did on a freshly generated code.
		m.msgs.clear()
		m.msgs.addText(helpText(m.th), m.th)
		m.qrHeaderIdx = len(m.msgs.blocks) // where addStyled(header) will land
		m.qrRendered = qr
		m.msgs.addStyled(header, true)
		m.msgs.addStyled(qr, false)
		m.msgs.vp.GotoBottom()
		return
	}

	// Same QR: countdown tick. Update only the header block in place so the
	// help/error text the user printed stays on screen and the QR stays put.
	if m.qrHeaderIdx < 0 || m.qrHeaderIdx >= len(m.msgs.blocks) {
		return // pane was reorganized; nothing safe to patch
	}
	b := &m.msgs.blocks[m.qrHeaderIdx]
	if strings.HasPrefix(b.text, "\n\n=== ") && strings.HasPrefix(header, "\n\n=== ") {
		b.text = header
		m.msgs.refresh()
	}
}

// ---- commands back to the backend -----------------------------------------

// sendCommand queues a backend command. The send happens inside a tea.Cmd so a
// full CommandChannel can never block the event loop.
func (m uiModel) sendCommand(name string, params []string) tea.Cmd {
	sm := m.sm
	return func() tea.Msg {
		sm.CommandChannel <- messages.Command{Name: name, Params: params}
		return nil
	}
}

// ---- View ------------------------------------------------------------------

// View composes the whole screen: outer frame, top bar, chat title rule,
// sidebar | message pane, input line.
func (m uiModel) View() string {
	if !m.ready {
		return "WhatsCLI — resizing… (terminal too small)"
	}
	th := m.th
	innerW := m.width - 2
	width := m.width

	sbW := m.side.width
	msgW := innerW - sbW - 1
	mainH := m.side.height - 2

	lines := make([]string, 0, m.height)
	lines = append(lines, frameTop(width, th))
	lines = append(lines, frameSide(fit(statusBarText(m.statusInfo, innerW, th), innerW), th))
	lines = append(lines, ruleTitledRight(width, m.msgs.title, th))

	sideLines := m.side.render(m.focus, th)
	sideLine := func(i int) string {
		if i < len(sideLines) {
			return fit(sideLines[i], sbW)
		}
		return strings.Repeat(" ", sbW)
	}
	msgLines := strings.Split(m.msgs.view(), "\n")
	for i := 0; i < mainH; i++ {
		var right string
		if i < len(msgLines) {
			right = msgLines[i]
		}
		lines = append(lines, th.Border.Render("│")+
			sideLine(i)+
			vRuleFocused(m.focus == focusMessages, th)+
			fit(right, msgW)+
			th.Border.Render("│"))
	}

	// The rule and the input line stop at the sidebar, so the chat list keeps
	// running to the bottom of the screen.
	lines = append(lines, th.Border.Render("│")+sideLine(mainH)+
		th.Border.Render("├"+strings.Repeat("─", msgW)+"┤"))
	lines = append(lines, th.Border.Render("│")+sideLine(mainH+1)+
		th.Border.Render("│")+m.renderInput(msgW)+th.Border.Render("│"))
	lines = append(lines, frameBottom(width, th))
	frame := strings.Join(lines, "\n")
	if m.picker != nil {
		frame = overlayCenter(frame, m.picker.render(th), width, m.height)
	}

	// Paint the theme background across the whole surface so a palette carries
	// even on terminals whose own colors do not match (a light theme on a dark
	// terminal, say). The input row sits on input_background instead of the
	// main surface; the pass runs after the overlay so picker rows are covered
	// too, and overlayCenter preserves the row count.
	lines = strings.Split(frame, "\n")
	for i, line := range lines {
		if i == len(lines)-2 { // the input row, directly above the bottom rule
			lines[i] = paintLine(line, width, th.inputSeq)
			continue
		}
		lines[i] = paintLine(line, width, th.backgroundSeq)
	}
	return strings.Join(lines, "\n")
}

// renderInput renders the input line with the configured input background
// filling the full width, like tview's InputField did.
func (m uiModel) renderInput(width int) string {
	view := ansi.Truncate(m.input.View(), width, "")
	if w := lipgloss.Width(view); w < width {
		view += strings.Repeat(" ", width-w)
	}
	return m.th.InputTextStyle.Render(view)
}

// ---- terminal-mode commands -------------------------------------------------

// enableMouseCmd turns on mouse reporting at runtime. Mouse is off by default
// (decision D4) so terminal text selection keeps working.
func enableMouseCmd() tea.Cmd {
	return func() tea.Msg { return tea.EnableMouseCellMotion() }
}

// disableMouseCmd turns mouse reporting back off.
func disableMouseCmd() tea.Cmd {
	return func() tea.Msg { return tea.DisableMouse() }
}

// openFileCmd opens a file with the platform handler (formerly UiHandler.OpenFile).
func openFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		_ = open.Run(path)
		return nil
	}
}
