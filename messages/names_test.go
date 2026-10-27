package messages

import (
	"container/heap"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestContactDisplayNamePrefersHumanNames(t *testing.T) {
	jid := types.NewJID("263771234567", types.DefaultUserServer)

	tests := []struct {
		name    string
		contact types.ContactInfo
		want    string
	}{
		{"address book name wins", types.ContactInfo{FullName: "Hope Mum", PushName: "Hope", BusinessName: "Hope Inc"}, "Hope Mum"},
		{"push name when no address book entry", types.ContactInfo{PushName: "Sly"}, "Sly"},
		{"business name when only that is known", types.ContactInfo{BusinessName: "CVR BOT"}, "CVR BOT"},
		{"first name as a last human resort", types.ContactInfo{FirstName: "Swazie"}, "Swazie"},
		{"redacted phone before raw number", types.ContactInfo{RedactedPhone: "+263∙∙∙∙∙∙∙25"}, "+263∙∙∙∙∙∙∙25"},
		{"raw number when nothing is known", types.ContactInfo{}, "263771234567"},
		{"whitespace is not a name", types.ContactInfo{FullName: "  ", PushName: "Seth"}, "Seth"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contactDisplayName(jid, tc.contact); got != tc.want {
				t.Errorf("contactDisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyContactNamesRelabelsPhoneChats(t *testing.T) {
	sm, mock := newTestSessionManager(t)

	jid := types.NewJID("263777738511", types.DefaultUserServer)
	conv := &Conversation{JID: jid.String(), Name: jid.User, Preview: "Hello", LastMsgTime: 1000}
	heap.Push(&sm.priorityQueue, conv)
	sm.convByJID[conv.JID] = conv

	if err := sm.db.AddMessage(Message{
		Id: "m1", ChatId: conv.JID, ContactId: conv.JID, ContactName: jid.User,
		ContactShort: jid.User, Timestamp: 1000, Text: "Hello",
	}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	renamed := sm.applyContactNames(map[types.JID]types.ContactInfo{
		jid: {Found: true, PushName: "Truth Ndlovu"},
	})
	if renamed != 1 {
		t.Fatalf("applyContactNames() renamed %d chats, want 1", renamed)
	}

	if got := sm.convByJID[conv.JID].Name; got != "Truth Ndlovu" {
		t.Errorf("conversation name = %q, want %q", got, "Truth Ndlovu")
	}
	if len(mock.ChatLists) != 1 {
		t.Fatalf("expected one chat list update, got %d", len(mock.ChatLists))
	}
	if got := mock.ChatLists[0][0].Name; got != "Truth Ndlovu" {
		t.Errorf("chat list name = %q, want %q", got, "Truth Ndlovu")
	}

	// The stored history rows are relabelled too so the message pane and
	// /search stop showing the phone number.
	msgs, err := sm.db.GetMessages(conv.JID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ContactName != "Truth Ndlovu" || msgs[0].ContactShort != "Truth Ndlovu" {
		t.Errorf("stored message names = %+v, want Truth Ndlovu", msgs)
	}

	// Persisted for the next start.
	convs, err := sm.db.GetConversations()
	if err != nil {
		t.Fatalf("GetConversations: %v", err)
	}
	if len(convs) != 1 || convs[0].Name != "Truth Ndlovu" {
		t.Errorf("persisted conversations = %+v", convs)
	}
}

func TestApplyContactNamesLeavesGroupsAndUnknownChatsAlone(t *testing.T) {
	sm, mock := newTestSessionManager(t)

	groupJID := "263716299218-1625848045@g.us"
	unknownJID := types.NewJID("263700000000", types.DefaultUserServer).String()

	group := &Conversation{JID: groupJID, Name: "Zimcurrent Affairs 46"}
	unknown := &Conversation{JID: unknownJID, Name: "263700000000"}
	for _, c := range []*Conversation{group, unknown} {
		heap.Push(&sm.priorityQueue, c)
		sm.convByJID[c.JID] = c
	}

	renamed := sm.applyContactNames(map[types.JID]types.ContactInfo{
		types.NewJID("263716299218-1625848045", types.GroupServer): {Found: true, FullName: "Should not win"},
	})

	if renamed != 0 {
		t.Errorf("applyContactNames() renamed %d chats, want 0", renamed)
	}
	if got := group.Name; got != "Zimcurrent Affairs 46" {
		t.Errorf("group name = %q, want it untouched", got)
	}
	if got := unknown.Name; got != "263700000000" {
		t.Errorf("unknown chat name = %q, want it untouched", got)
	}
	if len(mock.ChatLists) != 0 {
		t.Errorf("expected no chat list update, got %d", len(mock.ChatLists))
	}
}

func TestApplyContactNamesIsIdempotent(t *testing.T) {
	sm, mock := newTestSessionManager(t)

	jid := types.NewJID("263771234567", types.DefaultUserServer)
	conv := &Conversation{JID: jid.String(), Name: "Already Named"}
	heap.Push(&sm.priorityQueue, conv)
	sm.convByJID[conv.JID] = conv

	contacts := map[types.JID]types.ContactInfo{jid: {Found: true, FullName: "Already Named"}}
	if renamed := sm.applyContactNames(contacts); renamed != 0 {
		t.Errorf("first pass renamed %d chats, want 0", renamed)
	}
	if renamed := sm.applyContactNames(contacts); renamed != 0 {
		t.Errorf("second pass renamed %d chats, want 0", renamed)
	}
	if len(mock.ChatLists) != 0 {
		t.Errorf("expected no chat list updates, got %d", len(mock.ChatLists))
	}
}

func TestRenameConversationFromLiveMessage(t *testing.T) {
	sm, mock := newTestSessionManager(t)

	jid := types.NewJID("263776378708", types.DefaultUserServer)
	conv := &Conversation{JID: jid.String(), Name: jid.User}
	heap.Push(&sm.priorityQueue, conv)
	sm.convByJID[conv.JID] = conv

	sm.renameConversation(jid, "Seth/Keith")

	if got := sm.convByJID[conv.JID].Name; got != "Seth/Keith" {
		t.Errorf("conversation name = %q, want %q", got, "Seth/Keith")
	}
	if len(mock.ChatLists) != 1 {
		t.Fatalf("expected one chat list update, got %d", len(mock.ChatLists))
	}

	// whatsmeow signals "no push name" with a single dash: keep what we have.
	sm.renameConversation(jid, "-")
	if got := sm.convByJID[conv.JID].Name; got != "Seth/Keith" {
		t.Errorf("dash push name overwrote the name: got %q", got)
	}

	// Groups must not be renamed from contact push names.
	group := types.NewJID("263716299218-1625848045", types.GroupServer)
	groupConv := &Conversation{JID: group.String(), Name: "Zimcurrent Affairs 46"}
	heap.Push(&sm.priorityQueue, groupConv)
	sm.convByJID[groupConv.JID] = groupConv
	sm.renameConversation(group, "Should not win")
	if got := sm.convByJID[groupConv.JID].Name; got != "Zimcurrent Affairs 46" {
		t.Errorf("group name = %q, want it untouched", got)
	}
}
