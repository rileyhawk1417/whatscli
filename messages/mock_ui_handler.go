package messages

import "sync"

// MockUiHandler implements UiMessageHandler for testing.
// It captures all calls so tests can assert on them.
type MockUiHandler struct {
	mu sync.Mutex // backend callbacks can arrive from goroutines (e.g. PrintFile)

	Texts       []string
	Errors      []error
	Files       []string
	QRCodes     []string
	Statuses    []SessionStatus
	ChatLists   [][]*Conversation
	ChatSets    [][]Chat
	Screens     [][]Message
	Messages    []Message
	HelpCalled  int
	ClearCalled int
	QuitCalled  int
	ColorCalled int
	CmdsCalled  int
}

func NewMockUiHandler() *MockUiHandler {
	return &MockUiHandler{}
}

func (m *MockUiHandler) NewMessage(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, msg)
}
func (m *MockUiHandler) NewScreen(msgs []Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Screens = append(m.Screens, msgs)
}
func (m *MockUiHandler) SetChats(chats []Chat) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatSets = append(m.ChatSets, chats)
}
func (m *MockUiHandler) UpdateChatList(c []*Conversation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatLists = append(m.ChatLists, c)
}
func (m *MockUiHandler) PrintError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors = append(m.Errors, err)
}
func (m *MockUiHandler) PrintText(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Texts = append(m.Texts, text)
}
func (m *MockUiHandler) PrintFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Files = append(m.Files, path)
}
func (m *MockUiHandler) PrintQR(qr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.QRCodes = append(m.QRCodes, qr)
}
func (m *MockUiHandler) SetStatus(s SessionStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Statuses = append(m.Statuses, s)
}
func (m *MockUiHandler) OpenFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Files = append(m.Files, path)
}
func (m *MockUiHandler) ShowColorList()                  { m.mu.Lock(); defer m.mu.Unlock(); m.ColorCalled++ }
func (m *MockUiHandler) Clear()                          { m.mu.Lock(); defer m.mu.Unlock(); m.ClearCalled++ }
func (m *MockUiHandler) UpdateQR(_ string, _ int, _ int) {}
func (m *MockUiHandler) PrintCommands()                  { m.mu.Lock(); defer m.mu.Unlock(); m.CmdsCalled++ }
func (m *MockUiHandler) PrintHelp()                      { m.mu.Lock(); defer m.mu.Unlock(); m.HelpCalled++ }
func (m *MockUiHandler) Quit()                           { m.mu.Lock(); defer m.mu.Unlock(); m.QuitCalled++ }

// ScreenMessages returns the messages pushed in the i-th NewScreen call. Safe
// to call while backend goroutines are still running.
func (m *MockUiHandler) ScreenMessages(i int) []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i >= len(m.Screens) {
		return nil
	}
	out := make([]Message, len(m.Screens[i]))
	copy(out, m.Screens[i])
	return out
}

// HasText reports whether the handler was asked to print text. Safe to call
// while backend goroutines are still running.
func (m *MockUiHandler) HasText(text string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.Texts {
		if t == text {
			return true
		}
	}
	return false
}
