package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/normen/whatscli/messages"
	"github.com/skratchdot/open-golang/open"
)

// uiBridge hands the tea.Program to the backend handler. The handler is created
// before the program exists (SessionManager.Init needs it), so the pointer is
// attached once the program is built.
type uiBridge struct {
	program *tea.Program
}

func (b *uiBridge) send(msg tea.Msg) {
	if b == nil || b.program == nil {
		return
	}
	b.program.Send(msg)
}

// UiHandler implements messages.UiMessageHandler. Every method translates a
// backend callback into a bubbletea message; no function here renders or
// mutates UI state directly. Rendering (even of the help text and the colour
// list) happens in the model, so a theme change applies to output the backend
// requested afterwards.
type UiHandler struct {
	bridge *uiBridge
}

func (u UiHandler) NewMessage(msg messages.Message) {
	u.bridge.send(msgNewMessage{msg: msg})
}

func (u UiHandler) NewScreen(msgs []messages.Message) {
	u.bridge.send(msgScreen{msgs: msgs})
}

// SetChats is a legacy no-op kept for UiMessageHandler interface compatibility.
func (u UiHandler) SetChats(ids []messages.Chat) {}

func (u UiHandler) UpdateChatList(pq []*messages.Conversation) {
	u.bridge.send(msgChatList{pq: pq})
}

func (u UiHandler) PrintError(err error) {
	u.bridge.send(msgError{err: err})
}

func (u UiHandler) PrintText(msg string) {
	u.bridge.send(msgText{text: msg})
}

// PrintFile runs the external image renderer off the event loop and pushes the
// captured ANSI output back as a message.
func (u UiHandler) PrintFile(path string) {
	go func() {
		text, err := captureImageOutput(path)
		u.bridge.send(msgImageText{text: text, err: err})
	}()
}

func (u UiHandler) PrintQR(qr string) {
	u.bridge.send(msgStyled{text: qr + "\n"})
}

func (u UiHandler) SetStatus(status messages.SessionStatus) {
	u.bridge.send(msgSession{status: status})
}

func (u UiHandler) OpenFile(path string) {
	open.Run(path)
}

func (u UiHandler) ShowColorList() {
	u.bridge.send(msgColorList{})
}

func (u UiHandler) Clear() {
	u.bridge.send(msgClear{})
}

func (u UiHandler) PrintHelp() {
	u.bridge.send(msgHelp{})
}

func (u UiHandler) PrintCommands() {
	u.bridge.send(msgHelp{})
}

func (u UiHandler) Quit() {
	u.bridge.send(msgQuit{})
}

func (u UiHandler) UpdateQR(qr string, attempt int, timeout int) {
	u.bridge.send(msgUpdateQR{qr: qr, attempt: attempt, timeout: timeout})
}
