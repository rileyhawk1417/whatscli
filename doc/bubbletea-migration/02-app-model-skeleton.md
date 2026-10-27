# Task 02 — App model, Update/View skeleton, program lifecycle

**Depends on:** 01, decisions D3 (widgets), D5 (layout decision)
**Blocks:** 03, 04, 05 | **Gate: M1**

## Goal
Replace `tview.Application` lifecycle with `tea.Program`. A static skeleton
renders with the same 3-row layout; Ctrl+c quits cleanly.

## Current state (what is replaced)
- `main.go`: `ctx.App = tview.NewApplication()`, grid layout construction,
  `ctx.App.Run()`, `ctx.App.SetFocus(ctx.TextInput)`.
- `app_context.go`: widget fields (`App`, `TextView`, `TextInput`, `TopBar`,
  `LeftPane`, `ChatTable`, `GroupTable`, `StatusTable`) — replaced by model
  state fields.

## Steps
1. Define `type uiModel struct` with the state currently scattered around:
   - `CurrentReceiver messages.Chat`, `CurRegions []messages.Message`,
     `AllChats []*messages.Conversation`, `ChatLimit int`,
     `MouseState bool` — migrate, don't reimplement.
   - New: `focus focusArea` enum (`input | status | chats | groups | messages`)
     replacing tview focus tracking.
2. Replace `AppContext` widget fields with `*tea.Program` + model pointer;
   keep `SessionManager`, `KeyBindings`/keymap, `UiHandler` fields.
3. View: lipgloss `JoinVertical`/`JoinHorizontal` producing:
   row0 top bar (1 line) / row1 sidebar(w) + message pane / row2 input (1 line).
   Stub empty panes. Match `gridLayout.SetColumns(sideBarWidth, …)` semantics.
4. `main.go`: build model → `tea.NewProgram(model, tea.WithAltScreen()?)`
   — **decide altscreen vs inline** (tview runs fullscreen; altscreen matches).
   Pass `program.Send` capability into `SessionManager` bootstrap unchanged.
5. Ctrl+c / quit → send `disconnect` command then `tea.Quit` (parity with
   `handleQuit`).

## Acceptance
- [ ] Binary renders skeleton with title bar text, pane borders, input line.
- [ ] Window resize re-renders without corruption (`tea.WindowSizeMsg`).
- [ ] Quit sends backend `disconnect` and exits without deadlock.
- [ ] `messages/` untouched; all backend tests pass.

## Risks
- Deadlock: old code used `QueueUpdateDraw` from goroutines; new code must
  never Send from inside `Update()` synchronously — document the rule.