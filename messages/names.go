package messages

// names.go — resolving phone numbers to human names.
//
// The chat list is built from two sources that arrive at different times:
// the conversation rows (history sync, message events) and the contact store.
// WhatsApp fills the contact store *after* the first chat list exists — the
// address book arrives through app state sync and push names through history
// sync — so every chat created in between is labelled with its raw phone
// number and stays that way until something changes it. These helpers re-label
// those chats once the names are known.

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
)

// contactDisplayName picks the friendliest label a contact row offers: the
// name from the user's address book first, then the name the contact chose for
// themselves, then a verified business name. Only when nothing is known do we
// fall back to the phone number.
func contactDisplayName(jid types.JID, contact types.ContactInfo) string {
	for _, candidate := range []string{contact.FullName, contact.PushName, contact.BusinessName, contact.FirstName} {
		if name := strings.TrimSpace(candidate); name != "" {
			return name
		}
	}
	if phone := strings.TrimSpace(contact.RedactedPhone); phone != "" {
		return phone
	}
	return jid.User
}

// refreshConversationNames re-labels all stored one-to-one chats from the
// contact store. It is cheap (one query) and safe to call repeatedly; it only
// touches chats whose resolved name actually differs.
func (sm *SessionManager) refreshConversationNames() {
	client := sm.getClient()
	if client == nil || client.Store == nil || client.Store.Contacts == nil {
		return
	}
	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("failed to read contacts for chat names: %v", err))
		return
	}
	sm.applyContactNames(contacts)
}

// applyContactNames renames one-to-one conversations using the supplied contact
// map and returns how many chats were renamed. Group chats are left alone —
// their names come from the group metadata, not from contacts.
func (sm *SessionManager) applyContactNames(contacts map[types.JID]types.ContactInfo) int {
	var (
		renamed []Conversation
		chatIDs []string
	)

	sm.mu.Lock()
	for jidStr, conv := range sm.convByJID {
		jid, err := types.ParseJID(jidStr)
		if err != nil || jid.Server != types.DefaultUserServer {
			continue
		}
		contact, ok := contacts[jid]
		if !ok {
			continue
		}
		name := contactDisplayName(jid, contact)
		if name == "" || name == conv.Name {
			continue
		}
		conv.Name = name
		renamed = append(renamed, *conv)
		chatIDs = append(chatIDs, jidStr)
	}
	// Taking a snapshot here means the UI sees the new names in the same
	// refresh that reports them; no name affects the sort order.
	list := sm.snapshotPQ()
	sm.mu.Unlock()

	if len(renamed) == 0 {
		return 0
	}

	// Database writes happen outside the lock (see concurrency rules).
	for i, conv := range renamed {
		if err := sm.db.UpsertConversation(conv); err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("save chat name: %v", err))
			continue
		}
		if err := sm.db.UpdateChatContactName(chatIDs[i], conv.Name); err != nil {
			sm.uiHandler.PrintError(fmt.Errorf("rename stored messages: %v", err))
		}
	}
	sm.uiHandler.UpdateChatList(list)
	return len(renamed)
}

// renameConversation applies a single freshly learned name (a push name or a
// business name from a live message) without re-reading the whole contact
// store, which would be wasteful on busy accounts.
func (sm *SessionManager) renameConversation(jid types.JID, name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == "-" || jid.Server != types.DefaultUserServer {
		return
	}

	sm.mu.Lock()
	conv := sm.convByJID[jid.String()]
	if conv == nil || conv.Name == name {
		sm.mu.Unlock()
		return
	}
	conv.Name = name
	updated := *conv
	list := sm.snapshotPQ()
	sm.mu.Unlock()

	if err := sm.db.UpsertConversation(updated); err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("save chat name: %v", err))
	}
	if err := sm.db.UpdateChatContactName(jid.String(), name); err != nil {
		sm.uiHandler.PrintError(fmt.Errorf("rename stored messages: %v", err))
	}
	sm.uiHandler.UpdateChatList(list)
}
