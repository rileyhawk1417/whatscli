package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

// testTheme returns a zero-value theme: lipgloss zero styles render plain text,
// which keeps layout assertions readable.
func testTheme() *theme { return &theme{} }

// testModel builds a model big enough to render, with a live command channel.
func testModel(t *testing.T) uiModel {
	t.Helper()
	sm := &messages.SessionManager{CommandChannel: make(chan messages.Command, 16)}
	km := keyMap{
		global:          map[string]action{"ctrl+p": actToggleMouse, "backspace": actCommandHelp},
		messages:        map[string]action{},
		backspaceIsHelp: true,
	}
	m := newUIModel(testTheme(), sm, km)
	m.resize(100, 30)
	return m
}

func TestViewGeometry(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {80, 24}, {40, 12}, {200, 60}} {
		m := testModel(t)
		m.resize(size[0], size[1])
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) != size[1] {
			t.Fatalf("%dx%d: got %d lines, want %d", size[0], size[1], len(lines), size[1])
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w != size[0] {
				t.Fatalf("%dx%d: line %d is %d cells wide, want %d: %q", size[0], size[1], i, w, size[0], line)
			}
		}
	}
}

func TestViewTooSmall(t *testing.T) {
	m := testModel(t)
	m.resize(10, 4)
	if !strings.Contains(m.View(), "resizing") {
		t.Fatalf("expected placeholder for undersized terminal, got %q", m.View())
	}
}

func TestCanonicalKey(t *testing.T) {
	valid := map[string]string{
		"Ctrl+W":     "ctrl+w",
		"ctrl+w":     "ctrl+w",
		"Ctrl+Space": "ctrl+@",
		"Ctrl+?":     "backspace",
		"Ctrl+Q":     "ctrl+q",
		"Tab":        "tab",
		"Esc":        "esc",
		"Enter":      "enter",
		"Space":      " ",
		"Up":         "up",
		"PgUp":       "pgup",
		"PgDn":       "pgdown",
		"d":          "d",
		"G":          "G",
		"?":          "?",
	}
	for spec, want := range valid {
		got, err := canonicalKey(spec)
		if err != nil {
			t.Errorf("canonicalKey(%q) returned error: %v", spec, err)
			continue
		}
		if got != want {
			t.Errorf("canonicalKey(%q) = %q, want %q", spec, got, want)
		}
	}

	invalid := []string{"", "Ctrl+", "Ctrl+F1", "Shift+Up", "NotAKey"}
	for _, spec := range invalid {
		if got, err := canonicalKey(spec); err == nil {
			t.Errorf("canonicalKey(%q) = %q, want error", spec, got)
		}
	}
}

func TestBuildKeyMapDefaults(t *testing.T) {
	km, err := buildKeyMap(config.Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+W",
		FocusInput:      "Ctrl+T",
		FocusChats:      "Ctrl+N",
		Copyuser:        "Ctrl+C",
		Pasteuser:       "Ctrl+V",
		CommandRead:     "Ctrl+R",
		CommandBacklog:  "Ctrl+B",
		CommandConnect:  "Ctrl+L",
		CommandQuit:     "Ctrl+Q",
		CommandHelp:     "Ctrl+?",
		MessageDownload: "d",
		MessageOpen:     "o",
		MessageShow:     "s",
		MessageUrl:      "u",
		MessageInfo:     "i",
		MessageRevoke:   "r",
	})
	if err != nil {
		t.Fatalf("buildKeyMap: %v", err)
	}
	cases := []struct {
		key  string
		area focusArea
		want action
	}{
		{"ctrl+w", focusInput, actFocusMessages},
		{"tab", focusInput, actSwitchPanels},
		{"backspace", focusInput, actNone}, // editable: Ctrl+? == Backspace byte
		{"backspace", focusMessages, actCommandHelp},
		{"ctrl+p", focusChats, actToggleMouse},
		{"d", focusMessages, actMessageDownload},
		{"d", focusChats, actNone}, // message actions are pane-scoped
	}
	for _, c := range cases {
		if got := km.lookup(c.key, c.area); got != c.want {
			t.Errorf("lookup(%q, %v) = %v, want %v", c.key, c.area, got, c.want)
		}
	}
}

func TestBuildKeyMapReportsInvalid(t *testing.T) {
	if _, err := buildKeyMap(config.Keymap{SwitchPanels: "Ctrl+Meta+Q"}); err == nil {
		t.Fatal("expected error for unsupported binding")
	}
}

func TestSubmitInputSendsCommand(t *testing.T) {
	m := testModel(t)
	m.input.SetValue("/read")

	updated, cmd := m.submitInput()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if msg := cmd(); msg != nil {
		t.Fatalf("unexpected message %v", msg)
	}
	select {
	case c := <-m.sm.CommandChannel:
		if c.Name != "read" {
			t.Fatalf("sent %q, want read", c.Name)
		}
	default:
		t.Fatal("no command queued")
	}
	if updated.(uiModel).input.Value() != "" {
		t.Fatal("input was not cleared")
	}
}

func TestSubmitInputPlainTextSendsMessage(t *testing.T) {
	m := testModel(t)
	m.currentChat = messages.Chat{Id: "123@s.whatsapp.net", Name: "Ada"}
	m.input.SetValue("hello there")

	_, cmd := m.submitInput()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	cmd()
	c := <-m.sm.CommandChannel
	if c.Name != "send" || len(c.Params) != 2 || c.Params[0] != "123@s.whatsapp.net" || c.Params[1] != "hello there" {
		t.Fatalf("unexpected command %+v", c)
	}
}

func TestNewScreenRendersMessages(t *testing.T) {
	m := testModel(t)
	updated, _ := m.Update(msgScreen{msgs: []messages.Message{
		{Id: "1", ContactName: "Ada", Text: "hi", Timestamp: 1600000000},
		{Id: "2", ContactName: "Me", FromMe: true, Text: "hello", Timestamp: 1600000060},
	}})
	m = updated.(uiModel)

	if len(m.msgs.entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(m.msgs.entries))
	}
	view := m.View()
	if !strings.Contains(view, "hello") || !strings.Contains(view, "hi") {
		t.Fatalf("view does not contain messages:\n%s", view)
	}
}

func TestMessageSelectionHighlight(t *testing.T) {
	m := testModel(t)
	updated, _ := m.Update(msgNewMessage{msg: messages.Message{Id: "1", ContactName: "Ada", Text: "one"}})
	m = updated.(uiModel)
	updated, _ = m.Update(msgNewMessage{msg: messages.Message{Id: "2", ContactName: "Ada", Text: "two"}})
	m = updated.(uiModel)

	m.focus = focusMessages
	m.msgs.moveSelection(-1) // nothing selected yet → jump to the newest message
	if got := m.msgs.selectedMessage(); got == nil || got.Id != "2" {
		t.Fatalf("expected message 2 selected, got %+v", got)
	}
	m.msgs.moveSelection(-1)
	if got := m.msgs.selectedMessage(); got == nil || got.Id != "1" {
		t.Fatalf("expected message 1 selected, got %+v", got)
	}
	m.msgs.clearSelection()
	for _, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(line, "\x1b[7m") {
			t.Fatalf("highlight survived clearSelection: %q", line)
		}
	}
}

func TestANSIPassthroughIsNotRewrapped(t *testing.T) {
	m := testModel(t)
	qr := "\x1b[47m  \x1b[0m\x1b[40m  \x1b[0m\x1b[47m  \x1b[0m"
	m.msgs.addStyled(qr, false)
	if len(m.msgs.lines) != 1 {
		t.Fatalf("expected 1 passthrough line, got %d", len(m.msgs.lines))
	}
	if !strings.Contains(m.msgs.lines[0].styled, "\x1b[47m") {
		t.Fatal("ANSI escapes were stripped from the passthrough block")
	}
}

func TestSidebarSortAndSplit(t *testing.T) {
	s := newSidebar(batchSize)
	s.width, s.height = 30, 20
	s.setConversations([]*messages.Conversation{
		{JID: "a@s.whatsapp.net", Name: "A", LastMsgTime: 100},
		{JID: "12345@g.us", Name: "Group", LastMsgTime: 200},
		{JID: messages.STATUSSUFFIX, Name: "Status", LastMsgTime: 50},
		{JID: "b@s.whatsapp.net", Name: "B", LastMsgTime: 300, IsPinned: true},
		{JID: "arch@s.whatsapp.net", Name: "Archived", LastMsgTime: 999, IsArchived: true},
	})

	if got := s.chats[0].Name; got != "B" {
		t.Errorf("pinned chat should sort first, got %q", got)
	}
	if len(s.chats) != 2 {
		t.Errorf("got %d chats, want 2 (archived skipped)", len(s.chats))
	}
	if len(s.groups) != 1 || len(s.status) != 1 {
		t.Errorf("group/status split wrong: %d groups, %d status", len(s.groups), len(s.status))
	}
	if s.chats[1].Name != "A" {
		t.Errorf("time sort wrong: %q", s.chats[1].Name)
	}
}

func TestSidebarSelectionSurvivesRefresh(t *testing.T) {
	s := newSidebar(batchSize)
	s.width, s.height = 30, 20
	s.setConversations([]*messages.Conversation{
		{JID: "a@s.whatsapp.net", Name: "A", LastMsgTime: 100},
		{JID: "b@s.whatsapp.net", Name: "B", LastMsgTime: 200},
	})
	s.selChat = 0
	if got := s.selectedConversation(focusChats); got == nil || got.Name != "B" {
		t.Fatalf("setup: expected B selected, got %+v", got)
	}

	// New message on A reorders the list; the cursor must stay on B.
	s.setConversations([]*messages.Conversation{
		{JID: "a@s.whatsapp.net", Name: "A", LastMsgTime: 500},
		{JID: "b@s.whatsapp.net", Name: "B", LastMsgTime: 200},
	})
	conv := s.selectedConversation(focusChats)
	if conv == nil || conv.Name != "B" {
		t.Fatalf("selection not preserved, got %+v", conv)
	}
}

func TestSidebarRenderFillsHeight(t *testing.T) {
	s := newSidebar(batchSize)
	s.width, s.height = 30, 12
	s.setConversations([]*messages.Conversation{{JID: "a@s.whatsapp.net", Name: "A", LastMsgTime: 1}})
	lines := s.render(focusChats, testTheme())
	if len(lines) != 12 {
		t.Fatalf("got %d sidebar lines, want 12", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 30 {
			t.Fatalf("line %d width %d, want 30", i, w)
		}
	}
}

func TestCycleFocusSkipsEmptyStatus(t *testing.T) {
	m := testModel(t)
	m.focus = focusInput
	m.cycleFocus()
	if m.focus != focusChats {
		t.Fatalf("got focus %v, want chats", m.focus)
	}
	m.cycleFocus()
	if m.focus != focusGroups {
		t.Fatalf("got focus %v, want groups", m.focus)
	}
	m.cycleFocus()
	if m.focus != focusMessages {
		t.Fatalf("got focus %v, want messages", m.focus)
	}
	// SwitchPanels leaves the message pane for the sidebar, while Tab inside the
	// message pane goes to the input line (covered by the key handler).
	m.cycleFocus()
	if m.focus != focusChats {
		t.Fatalf("got focus %v, want chats", m.focus)
	}
}

func TestCtrlPReportsMouseState(t *testing.T) {
	m := testModel(t)
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(uiModel)
	if cmd == nil {
		t.Fatal("expected a mouse-mode command")
	}
	if !m.mouseEnabled {
		t.Fatal("mouse should be enabled")
	}
	if !strings.Contains(m.View(), "ENABLED") {
		t.Fatal("expected a notice in the message pane")
	}
}

func TestEmptyScreenShowsHint(t *testing.T) {
	m := testModel(t)
	m.currentChat = messages.Chat{Id: "a@s.whatsapp.net", Name: "A"}
	updated, _ := m.Update(msgScreen{msgs: nil})
	m = updated.(uiModel)
	if !strings.Contains(m.View(), "no messages") {
		t.Fatalf("expected empty-chat hint, got:\n%s", m.View())
	}
}

// Ctrl+? and Backspace are the same byte, so the default help binding must not
// swallow backspace while the user is typing.
func TestBackspaceEditsInputInsteadOfShowingHelp(t *testing.T) {
	m := testModel(t)
	if got := m.keys.lookup("backspace", focusInput); got != actNone {
		t.Fatalf("backspace in input resolved to %v, want actNone", got)
	}
	if got := m.keys.lookup("backspace", focusMessages); got != actCommandHelp {
		t.Fatalf("backspace outside the input resolved to %v, want actCommandHelp", got)
	}
}

// Free-form output (help, errors, QR codes) must scroll into view when the user
// is already at the bottom, but must not yank them away from history they are
// reading.
func TestFreeFormOutputFollowsBottom(t *testing.T) {
	m := testModel(t)
	m.resize(80, 24)

	m.msgs.addText(strings.Repeat("line\n", 100), m.th)
	if m.msgs.vp.YOffset != len(m.msgs.lines)-m.msgs.vp.Height {
		t.Fatalf("YOffset = %d, want %d (bottom)", m.msgs.vp.YOffset, len(m.msgs.lines)-m.msgs.vp.Height)
	}

	m.msgs.vp.SetYOffset(0)
	m.msgs.addText("another line", m.th)
	if m.msgs.vp.YOffset != 0 {
		t.Fatalf("YOffset = %d, want 0 (user had scrolled up)", m.msgs.vp.YOffset)
	}
}

// The QR countdown fires UpdateQR once a second. Ticks must not wipe the login
// screen, reset the input or steal focus — otherwise the login screen is
// untypeable. Regression test for the post-milestone fix in
// doc/bubbletea-migration/00-design-decisions.md.
func qrTestModel(t *testing.T) uiModel {
	t.Helper()
	sm := &messages.SessionManager{CommandChannel: make(chan messages.Command, 16)}
	km, err := buildKeyMap(config.Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+w",
		FocusInput:      "Ctrl+Space",
		FocusChats:      "Ctrl+e",
		Copyuser:        "Ctrl+c",
		Pasteuser:       "Ctrl+v",
		CommandBacklog:  "Ctrl+b",
		CommandRead:     "Ctrl+n",
		CommandConnect:  "Ctrl+r",
		CommandQuit:     "Ctrl+q",
		CommandHelp:     "Ctrl+?",
		MessageDownload: "d",
		MessageOpen:     "o",
		MessageShow:     "s",
		MessageUrl:      "u",
		MessageInfo:     "i",
		MessageRevoke:   "r",
	})
	if err != nil {
		t.Fatalf("buildKeyMap: %v", err)
	}
	m := newUIModel(testTheme(), sm, km)
	m.resize(80, 24)
	return m
}

func TestShowQRCountdownKeepsInput(t *testing.T) {
	m := qrTestModel(t)

	m.input.SetValue("typed by the user")
	m.showQR("QR-PAYLOAD", 1, 20)
	if m.qrRendered != "QR-PAYLOAD" || m.qrHeaderIdx == 0 {
		t.Fatalf("first render: qrRendered=%q qrHeaderIdx=%d", m.qrRendered, m.qrHeaderIdx)
	}
	blocksAfterBuild := len(m.msgs.blocks)
	inputValue, focus := m.input.Value(), m.focus

	// Tab away, then let three countdown ticks fire.
	upd, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = upd.(uiModel)
	if m.focus == focus {
		t.Fatalf("tab did not move focus away from the input")
	}
	for i := 19; i >= 17; i-- {
		m.showQR("QR-PAYLOAD", 1, i)
		if m.input.Value() != inputValue {
			t.Fatalf("countdown tick erased the input (%d)", i)
		}
		if m.focus != upd.(uiModel).focus {
			t.Fatalf("countdown tick changed focus (%d)", i)
		}
	}
	if len(m.msgs.blocks) != blocksAfterBuild {
		t.Fatalf("countdown ticks changed block count: got %d, want %d", len(m.msgs.blocks), blocksAfterBuild)
	}
	// The countdown line must actually count down.
	if line := m.msgs.blocks[m.qrHeaderIdx].text; !strings.Contains(line, "in 17s") {
		t.Fatalf("countdown line not updated in place: %q", line)
	}

	// User-printed content between QR codes must survive the next tick...
	m.msgs.addText("printed after the QR screen", m.th)
	m.showQR("QR-PAYLOAD", 1, 16)
	if got := m.msgs.blocks[len(m.msgs.blocks)-1].text; got != "printed after the QR screen" {
		t.Fatalf("tick wiped user content: last block is %q", got)
	}

	// ...but a fresh QR code rebuilds the screen like the old handler did.
	m.input.SetValue("x")
	m.showQR("QR-PAYLOAD-2", 2, 20)
	if m.qrRendered != "QR-PAYLOAD-2" {
		t.Fatalf("new QR code not rebuilt (qrRendered=%q)", m.qrRendered)
	}
	if len(m.msgs.blocks) != blocksAfterBuild {
		t.Fatalf("new QR code screen not rebuilt: %d blocks, want %d (cleared+rebuilt)", len(m.msgs.blocks), blocksAfterBuild)
	}
	for _, b := range m.msgs.blocks {
		if b.kind == blockText && strings.Contains(b.text, "printed after the QR screen") {
			t.Fatalf("new QR code rebuild kept user-printed text")
		}
	}
	if m.qrHeaderIdx >= len(m.msgs.blocks) {
		t.Fatalf("qrHeaderIdx out of range: %d >= %d", m.qrHeaderIdx, len(m.msgs.blocks))
	}
}
