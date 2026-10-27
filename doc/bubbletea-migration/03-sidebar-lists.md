# Task 03 — Sidebar: status / chats / groups lists

**Depends on:** 02, D3 (list widget choice)
**Blocks:** 06 | **Contributes to: M2**

## Goal
Port the three-pane sidebar (Status, Chats, Groups) with selection, focus
ring, infinite scroll, and the pinned/unread/timestamp row rendering.

## Current state (what is replaced)
- `ui_layout.go` — all three `tview.Table` constructions,
  `setActiveTable` focus highlighting, selection handlers
  (`openFromStatus/Chat/Group`), infinite scroll trigger
  (`row >= rowCount-5 && ChatLimit < len(AllChats)` → `ChatLimit += batchSize`).
- `ui_render.go → RenderChatTable` — row building: skip archived,
  name fallback to JID, unread `(n)` in `Colors.UnreadCount`, 📌 prefix,
  right-aligned `formatRelativeTime`, color by pane
  (`ListContact`/`ListGroup`, yellow status), selection restore by JID +
  `RenderingList` guard.
- `ui_handler.go → UpdateChatList` — sort (pinned first, then LastMsgTime desc).

## Steps
1. Model: `chatRows` derived list + `statusSel/chatSel/groupSel int` JIDs.
2. Render function per pane (per D3, likely custom strings, not
   `bubbles/table`): name column + right-aligned relative time column within
   pane width. Focused pane gets border/reverse highlight on selected row.
3. Selection → `SetDisplayedChat(chat)` port (sends `select` command) —
   on Enter AND on arrow movement (parity with `SetSelectionChangedFunc`).
4. Infinite scroll: when selection reaches last visible row and hidden chats
   remain, grow `ChatLimit` and re-render (guard: only when moving down).
5. Selection restore is trivial now — model state, no JID-snapshot dance, no
   `RenderingList` guard needed (document removal).
6. Cross-pane boundary behavior (port of handlers): chats↑ at top → status;
   chats↓ at bottom → groups; groups↑ at top → chats; status↓ → chats
   (`handleChatPanelUp/Down`, `handleGroupPanelUp/Down`,
   `handleStatusPanelDown`). Tab flow: status→chats→groups→messages→input
   (`handleStatusPanelTab` etc.).

## Acceptance
- [ ] All three panes populate on `UpdateChatList`; ordering matches today
      (pinned first, then recency).
- [ ] Arrow keys traverse panes seamlessly; Tab cycles focus.
- [ ] Infinite scroll loads +50 chats at the bottom edge.
- [ ] Selecting a chat swaps message pane content.
- [ ] No ghost chat-switch when list re-renders (old `RenderingList` bug class).

## Risks
- Wide CJK/emoji names breaking column alignment — test with emoji names.