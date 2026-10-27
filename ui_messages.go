package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

// paneBlock is one logical piece of pane content: a message, a line of
// free-form text, or a pre-rendered ANSI block (QR code, image output).
type paneBlock struct {
	styled string // may contain newlines
	msg    int    // index into entries, -1 for non-selectable output
	wrap   bool   // false for ANSI passthrough (decision D6)
}

// paneLine is one rendered display line derived from a block.
type paneLine struct {
	styled string
	plain  string
	msg    int
}

// messagesPane replaces tview.TextView plus its region/highlight machinery.
// Selection is an explicit index instead of a region id string, so the
// RenderingList-style guards the old implementation needed are gone.
type messagesPane struct {
	vp       viewport.Model
	ready    bool
	entries  []messages.Message // selectable messages, formerly ctx.CurRegions
	blocks   []paneBlock
	lines    []paneLine
	start    []int // first display line of each entry
	selected int   // -1 when nothing is selected
	width    int
	height   int
	title    string
	th       *theme
}

func newMessagesPane(th *theme) messagesPane {
	vp := viewport.New(0, 0)
	vp.KeyMap = viewport.KeyMap{} // scrolling is driven by the configurable keymap
	return messagesPane{vp: vp, selected: -1, th: th}
}

// setSize resizes the pane and re-flows existing content. Re-flow (not just
// re-clip) is why blocks are stored un-wrapped.
func (p *messagesPane) setSize(w, h int) {
	if w == p.width && h == p.height && p.ready {
		return
	}
	p.width, p.height = w, h
	p.vp.Width, p.vp.Height = w, h
	p.ready = true
	p.refresh()
}

// setTitle sets the title shown in the rule above the pane (formerly
// TextView.SetTitle).
func (p *messagesPane) setTitle(title string) { p.title = title }

// clear drops all content and selection.
func (p *messagesPane) clear() {
	p.entries = nil
	p.blocks = nil
	p.lines = nil
	p.start = nil
	p.selected = -1
	p.vp.SetContent("")
	p.vp.SetYOffset(0)
}

// setScreen replaces the pane with a full chat history (formerly NewScreen).
func (p *messagesPane) setScreen(msgs []messages.Message, th *theme) {
	p.clear()
	for i := range msgs {
		// Build the blocks directly and re-flow once: a backlog can hold
		// thousands of messages and refreshing per message is quadratic.
		p.appendMessage(msgs[i], th)
	}
	p.refresh()
	p.vp.GotoBottom()
}

// addMessage appends a single message (formerly NewMessage). It only forces a
// scroll to the bottom when the user is already there.
func (p *messagesPane) addMessage(msg messages.Message, th *theme) {
	atBottom := p.atBottom()
	p.addMessageEntry(msg, th)
	if atBottom {
		p.vp.GotoBottom()
	}
}

// addMessageEntry appends one message, re-flows, and returns whether it was
// actually added.
func (p *messagesPane) addMessageEntry(msg messages.Message, th *theme) bool {
	if !p.appendMessage(msg, th) {
		return false
	}
	p.refresh()
	return true
}

// appendMessage records a message without touching the viewport content.
func (p *messagesPane) appendMessage(msg messages.Message, th *theme) bool {
	if strings.TrimSpace(msg.Text) == "" {
		return false // skip empty/protocol messages, as getMessagesString did
	}
	idx := len(p.entries)
	p.entries = append(p.entries, msg)
	p.blocks = append(p.blocks, paneBlock{styled: renderMessage(&msg, th), msg: idx, wrap: true})
	return true
}

// addText appends free-form output (help, errors, notices).
func (p *messagesPane) addText(text string, th *theme) {
	atBottom := p.atBottom()
	style := lipgloss.NewStyle().Foreground(p.th.Text)
	for _, raw := range strings.Split(text, "\n") {
		p.blocks = append(p.blocks, paneBlock{styled: style.Render(raw), msg: -1, wrap: true})
	}
	p.refresh()
	if atBottom {
		p.vp.GotoBottom()
	}
}

// addStyled appends pre-rendered ANSI. Wrapping is opt-in because QR codes and
// image output must never be re-flowed (decision D6).
func (p *messagesPane) addStyled(text string, wrap bool) {
	atBottom := p.atBottom()
	p.blocks = append(p.blocks, paneBlock{styled: text, msg: -1, wrap: wrap})
	p.refresh()
	if atBottom {
		p.vp.GotoBottom()
	}
}

// replaceWithText clears the pane and shows free-form text.
func (p *messagesPane) replaceWithText(text string, th *theme) {
	p.clear()
	p.addText(text, th)
	p.vp.GotoBottom()
}

// atBottom reports whether the viewport is scrolled to the end.
func (p *messagesPane) atBottom() bool {
	return len(p.lines) <= p.height || p.vp.YOffset >= len(p.lines)-p.height
}

// refresh re-flows every block into display lines, then rebuilds the viewport
// content with the current selection applied.
func (p *messagesPane) refresh() {
	p.lines = p.lines[:0]
	p.start = p.start[:0]

	for _, b := range p.blocks {
		if b.msg >= 0 {
			for len(p.start) <= b.msg {
				p.start = append(p.start, len(p.lines))
			}
			p.start[b.msg] = len(p.lines)
		}
		var raw []string
		if b.wrap && p.width > 0 {
			raw = wrapText(b.styled, p.width)
		} else {
			raw = strings.Split(b.styled, "\n")
		}
		for _, line := range raw {
			p.lines = append(p.lines, paneLine{styled: line, plain: ansi.Strip(line), msg: b.msg})
		}
	}

	if !p.ready {
		return
	}
	rendered := make([]string, len(p.lines))
	for i, line := range p.lines {
		if line.msg >= 0 && line.msg == p.selected {
			rendered[i] = p.th.Selected.Render(padTo(line.plain, p.width))
		} else {
			rendered[i] = line.styled
		}
	}
	p.vp.SetContent(strings.Join(rendered, "\n"))
}

// selectedMessage returns the message under the selection cursor.
func (p *messagesPane) selectedMessage() *messages.Message {
	if p.selected < 0 || p.selected >= len(p.entries) {
		return nil
	}
	return &p.entries[p.selected]
}

// moveSelection moves the message cursor by delta, clamping at the ends.
func (p *messagesPane) moveSelection(delta int) bool {
	if len(p.entries) == 0 {
		return false
	}
	if p.selected < 0 {
		// Parity with GetOffsetMsgId: moving down from "nothing selected" lands
		// on the oldest message, moving up on the newest.
		if delta > 0 {
			p.selected = 0
		} else {
			p.selected = len(p.entries) - 1
		}
		p.refresh()
		p.scrollToSelection()
		return true
	}
	next := clamp(p.selected+delta, 0, len(p.entries)-1)
	if next == p.selected {
		return false
	}
	p.selected = next
	p.refresh()
	p.scrollToSelection()
	return true
}

// selectFirst / selectLast jump to the ends of the history.
func (p *messagesPane) selectFirst() {
	if len(p.entries) == 0 {
		return
	}
	p.selected = 0
	p.refresh()
	p.scrollToSelection()
}

func (p *messagesPane) selectLast() {
	if len(p.entries) == 0 {
		return
	}
	p.selected = len(p.entries) - 1
	p.refresh()
	p.scrollToSelection()
}

// entryAtLine returns the message index whose first display line is the closest
// at or before line, or -1 when the line holds no message.
func (p *messagesPane) entryAtLine(line int) int {
	if line < 0 || line >= len(p.lines) {
		return -1
	}
	if p.lines[line].msg >= 0 {
		return p.lines[line].msg
	}
	// Wrapped continuation lines carry their message index already, so a
	// non-message line here means free-form output.
	return -1
}

// selectEntry selects a specific message index (mouse click).
func (p *messagesPane) selectEntry(idx int) {
	if idx < 0 || idx >= len(p.entries) {
		return
	}
	p.selected = idx
	p.refresh()
	p.scrollToSelection()
}

// clearSelection drops the highlight and returns to the bottom of the history
// (formerly ResetMsgSelection).
func (p *messagesPane) clearSelection() {
	if p.selected >= 0 {
		p.selected = -1
		p.refresh()
	}
	p.vp.GotoBottom()
}

// scrollToSelection makes the selected message visible.
func (p *messagesPane) scrollToSelection() {
	if p.selected < 0 || p.selected >= len(p.start) {
		return
	}
	line := p.start[p.selected]
	off := p.vp.YOffset
	if line < off {
		off = line
	} else if line >= off+p.height {
		off = line - p.height + 1
	}
	p.vp.SetYOffset(clamp(off, 0, max(0, len(p.lines)-p.height)))
}

// view renders the pane content, padded to exactly height lines.
func (p *messagesPane) view() string {
	if p.height <= 0 {
		return ""
	}
	lines := strings.Split(p.vp.View(), "\n")
	for len(lines) < p.height {
		lines = append(lines, "")
	}
	if len(lines) > p.height {
		lines = lines[:p.height]
	}
	for i := range lines {
		lines[i] = fit(lines[i], p.width)
	}
	return strings.Join(lines, "\n")
}

// captureImageOutput runs the configured ShowCommand against path and returns
// its ANSI output. All path hardening from the previous implementation is kept
// (audit S-2: symlink and directory-escape protection).
func captureImageOutput(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %v", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("file not found: %v", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %v", err)
	}
	downloadDir := filepath.Clean(config.Config.General.DownloadPath)
	previewDir := filepath.Clean(config.Config.General.PreviewPath)
	if !strings.HasPrefix(resolvedPath, downloadDir) && !strings.HasPrefix(resolvedPath, previewDir) {
		return "", fmt.Errorf("file path outside allowed directories")
	}

	cmdParts := strings.Split(config.Config.General.ShowCommand, " ")
	cmdParts = append(cmdParts, resolvedPath)
	var cmd *exec.Cmd
	switch len(cmdParts) {
	case 0:
		return "", fmt.Errorf("show command not configured")
	case 1:
		cmd = exec.Command(cmdParts[0])
	default:
		cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var out strings.Builder
	if _, err := io.Copy(&out, bufio.NewReader(stdout)); err != nil {
		_ = cmd.Wait()
		return out.String(), err
	}
	if err := cmd.Wait(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}
