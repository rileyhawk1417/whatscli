package main

import (
	"sort"
	"strings"

	"github.com/normen/whatscli/messages"
)

// sidebar holds the three chat lists (status, chats, groups) plus their
// independent cursors and scroll offsets. It replaces the three tview.Tables
// from ui_layout.go.
type sidebar struct {
	all   []*messages.Conversation
	limit int

	status []*messages.Conversation
	chats  []*messages.Conversation
	groups []*messages.Conversation

	selStatus, selChat, selGroup int
	offStatus, offChat, offGroup int
	width, height                int
}

func newSidebar(limit int) sidebar {
	return sidebar{limit: limit}
}

// setConversations replaces the backing list: it sorts (pinned first, then most
// recent), truncates to the current infinite-scroll limit, and splits the
// result into the three panes. Selection is preserved by JID so a list refresh
// never yanks the cursor to another chat.
func (s *sidebar) setConversations(pq []*messages.Conversation) {
	var prevStatus, prevChat, prevGroup string
	if c := s.selectedConversation(focusStatus); c != nil {
		prevStatus = c.JID
	}
	if c := s.selectedConversation(focusChats); c != nil {
		prevChat = c.JID
	}
	if c := s.selectedConversation(focusGroups); c != nil {
		prevGroup = c.JID
	}

	s.all = make([]*messages.Conversation, len(pq))
	copy(s.all, pq)
	sort.SliceStable(s.all, func(i, j int) bool {
		a, b := s.all[i], s.all[j]

		// Pinned check
		if a.IsPinned && !b.IsPinned {
			return true
		}
		if !a.IsPinned && b.IsPinned {
			return false
		}

		// Time check (descending)
		return a.LastMsgTime > b.LastMsgTime
	})

	s.rebuild()

	s.selStatus = restoreSelection(s.status, prevStatus, s.selStatus)
	s.selChat = restoreSelection(s.chats, prevChat, s.selChat)
	s.selGroup = restoreSelection(s.groups, prevGroup, s.selGroup)
}

// rebuild re-splits the visible slice of s.all into the three panes, skipping
// archived conversations exactly like RenderChatTable did.
func (s *sidebar) rebuild() {
	display := s.all
	if s.limit > 0 && len(display) > s.limit {
		display = display[:s.limit]
	}

	s.status = s.status[:0]
	s.chats = s.chats[:0]
	s.groups = s.groups[:0]
	for _, conv := range display {
		if conv.IsArchived {
			continue
		}
		switch {
		case conv.JID == messages.STATUSSUFFIX:
			s.status = append(s.status, conv)
		case strings.HasSuffix(conv.JID, messages.GROUPSUFFIX):
			s.groups = append(s.groups, conv)
		default:
			s.chats = append(s.chats, conv)
		}
	}
}

// restoreSelection finds the conversation with jid and returns its index,
// falling back to a clamped index when it is gone.
func restoreSelection(rows []*messages.Conversation, jid string, fallback int) int {
	if jid != "" {
		for i, c := range rows {
			if c.JID == jid {
				return i
			}
		}
	}
	if len(rows) == 0 {
		return 0
	}
	return clamp(fallback, 0, len(rows)-1)
}

// rows returns the row slice backing a pane.
func (s *sidebar) rows(pane focusArea) []*messages.Conversation {
	switch pane {
	case focusStatus:
		return s.status
	case focusChats:
		return s.chats
	case focusGroups:
		return s.groups
	}
	return nil
}

// selectedConversation returns the conversation under the cursor of a pane.
func (s *sidebar) selectedConversation(pane focusArea) *messages.Conversation {
	rows := s.rows(pane)
	var idx int
	switch pane {
	case focusStatus:
		idx = s.selStatus
	case focusChats:
		idx = s.selChat
	case focusGroups:
		idx = s.selGroup
	default:
		return nil
	}
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	return rows[idx]
}

// moveCursor moves the cursor inside a pane by delta, returning true when it
// actually moved. It also grows the infinite-scroll window when the cursor
// reaches the bottom of the chats pane (parity with openFromChat).
func (s *sidebar) moveCursor(pane focusArea, delta int) bool {
	rows := s.rows(pane)
	if len(rows) == 0 {
		return false
	}
	var sel *int
	switch pane {
	case focusStatus:
		sel = &s.selStatus
	case focusChats:
		sel = &s.selChat
	case focusGroups:
		sel = &s.selGroup
	default:
		return false
	}
	next := clamp(*sel+delta, 0, len(rows)-1)
	moved := next != *sel
	*sel = next

	if pane == focusChats && delta > 0 && next == len(rows)-1 && s.limit < len(s.all) {
		s.limit += batchSize
		s.rebuild()
	}
	return moved
}

// selectFirst / selectLast implement the cross-pane jumps of the old handlers.
func (s *sidebar) selectFirst(pane focusArea) {
	switch pane {
	case focusStatus:
		s.selStatus = 0
	case focusChats:
		s.selChat = 0
	case focusGroups:
		s.selGroup = 0
	}
}

func (s *sidebar) selectLast(pane focusArea) {
	switch pane {
	case focusStatus:
		s.selStatus = max(0, len(s.status)-1)
	case focusChats:
		s.selChat = max(0, len(s.chats)-1)
	case focusGroups:
		s.selGroup = max(0, len(s.groups)-1)
	}
}

// layout computes the content height of each pane for a given sidebar height.
// Three title rules plus one fixed status row are reserved first; the rest is
// split evenly between chats and groups.
func (s *sidebar) layout() (statusH, chatsH, groupsH int) {
	const rules = 3
	content := s.height - rules
	if content < 3 {
		content = 3
	}
	statusH = 1
	rest := content - statusH
	chatsH = rest / 2
	groupsH = rest - chatsH
	return
}

// render returns exactly s.height lines, each s.width cells wide.
func (s *sidebar) render(focus focusArea, th *theme) []string {
	if s.height <= 0 || s.width <= 0 {
		return nil
	}
	statusH, chatsH, groupsH := s.layout()

	lines := make([]string, 0, s.height)
	lines = append(lines, fit(ruleTitled(s.width, "Status", focus == focusStatus, th), s.width))
	lines = append(lines, s.renderPane(focusStatus, statusH, focus, th)...)
	lines = append(lines, fit(ruleTitled(s.width, "Chats", focus == focusChats, th), s.width))
	lines = append(lines, s.renderPane(focusChats, chatsH, focus, th)...)
	lines = append(lines, fit(ruleTitled(s.width, "Groups", focus == focusGroups, th), s.width))
	lines = append(lines, s.renderPane(focusGroups, groupsH, focus, th)...)

	// Guard against rounding drift.
	for len(lines) < s.height {
		lines = append(lines, fit("", s.width))
	}
	return lines[:s.height]
}

// renderPane renders one pane's visible rows, keeping the cursor on screen.
func (s *sidebar) renderPane(pane focusArea, height int, focus focusArea, th *theme) []string {
	rows := s.rows(pane)
	sel := s.selOf(pane)
	off := s.offsetOf(pane)

	if len(rows) == 0 {
		off = 0
	}
	if sel < off {
		off = sel
	}
	if sel >= off+height {
		off = sel - height + 1
	}
	off = clamp(off, 0, max(0, len(rows)-height))
	s.setOffset(pane, off)

	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := off + i
		if idx < 0 || idx >= len(rows) {
			out = append(out, fit("", s.width))
			continue
		}
		conv := rows[idx]
		selected := focus == pane && idx == sel
		color := th.ListContact
		switch pane {
		case focusGroups:
			color = th.ListGroup
		case focusStatus:
			color = statusColor
		}
		out = append(out, fit(renderChatRow(conv, selected, color, s.width, th), s.width))
	}
	return out
}

// selOf returns the cursor index for a pane.
func (s *sidebar) selOf(pane focusArea) int {
	switch pane {
	case focusStatus:
		return s.selStatus
	case focusChats:
		return s.selChat
	case focusGroups:
		return s.selGroup
	}
	return 0
}

// setSel moves a pane's cursor to idx.
func (s *sidebar) setSel(pane focusArea, idx int) {
	switch pane {
	case focusStatus:
		s.selStatus = idx
	case focusChats:
		s.selChat = idx
	case focusGroups:
		s.selGroup = idx
	}
}

// offsetOf returns the scroll offset for a pane.
func (s *sidebar) offsetOf(pane focusArea) int {
	switch pane {
	case focusStatus:
		return s.offStatus
	case focusChats:
		return s.offChat
	case focusGroups:
		return s.offGroup
	}
	return 0
}

// setOffset stores the scroll offset for a pane.
func (s *sidebar) setOffset(pane focusArea, off int) {
	switch pane {
	case focusStatus:
		s.offStatus = off
	case focusChats:
		s.offChat = off
	case focusGroups:
		s.offGroup = off
	}
}

// hitTest maps a sidebar-local row to a pane and row index. It returns ok=false
// for rows that are pane title rules.
func (s *sidebar) hitTest(y int) (focusArea, int, bool) {
	statusH, chatsH, groupsH := s.layout()

	if y == 0 {
		return focusStatus, -1, false
	}
	if y -= 1; y < statusH {
		return focusStatus, s.offStatus + y, true
	}
	y -= statusH

	if y == 0 {
		return focusChats, -1, false
	}
	if y -= 1; y < chatsH {
		return focusChats, s.offChat + y, true
	}
	y -= chatsH

	if y == 0 {
		return focusGroups, -1, false
	}
	if y -= 1; y < groupsH {
		return focusGroups, s.offGroup + y, true
	}
	return focusGroups, -1, false
}
