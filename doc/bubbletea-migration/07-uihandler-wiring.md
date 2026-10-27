# Task 07 — UiHandler rewiring to program.Send + state sync

**Depends on:** 02 (03–06 for full behavior) | **Blocks:** 08 | **Gate: M3**

## Goal
All 17 `UiMessageHandler` methods push bubbletea messages instead of
`QueueUpdateDraw`, keeping the `messages/` package contract untouched.

## Current state (what is replaced)
`ui_handler.go`: every method wraps work in
`go ctx.App.QueueUpdateDraw(func(){...})`. Target pattern:

```go
func (u UiHandler) NewMessage(msg messages.Message) {
    program.Send(uiMsgNewMessage{msg})
}
```

Update rules in the model apply changes and render. Concurrency note: the old
code needed `SetChangedFunc → App.Draw()`, `QueueUpdateDraw` from goroutines,
and `snapshotPQ()` discipline; bubbletea `Send` is goroutine-safe — simpler
and race-free by construction.

## Steps
1. Define msg types: `uiMsgNewMessage`, `uiMsgScreen`, `uiMsgChatList`,
   `uiMsgStatus`, `uiMsgPrintText`, `uiMsgPrintError`, `uiMsgQR`,
   `uiMsgUpdateQR{qr,attempt,timeout}`, `uiMsgClear`,
   `uiMsgQuit`, media messages (`PrintFile` → model runs pipeline from
   `PrintImage` port in 04).
2. Port each method 1:1; port `UpdateChatList` sort into update rule
   (pinned first, LastMsgTime desc — exact current comparator).
3. `SetStatus` → top bar re-render (port `UpdateStatusBar` incl. battery
   L/l S/s indicators and online/offline color).
4. `UpdateQR` port incl. refocus input + clearing.
5. `OpenFile` (`open.Run`) stays synchronous — no change.
6. Keep `SetChats` legacy no-op for interface compatibility.

## Acceptance
- [ ] All 17 methods implemented over Send + Update; grep shows zero
      `QueueUpdateDraw`/`tview` refs in `ui_handler.go`.
- [ ] Receiving messages while scrolled-up does not yank scroll to bottom
      unless already at bottom (04 rule).
- [ ] `messages` package diff = zero lines.
- [ ] `make test -race` green with MockUiHandler tests untouched.

## Risks
- Ordering of Send msg types vs. current serialized `QueueUpdateDraw` calls —
  bubbletea processes messages sequentially via Update; verify no dropped/
  interleaved updates in stress test (task 09).