# Design Decisions — tview → Bubbletea Migration

Open questions that must be resolved **before** the corresponding task starts.
Each entry records the options considered and the chosen resolution once decided.

## D1. Color markup strategy

**Problem:** Every print helper (`PrintHelp`, `PrintError`, `UpdateStatusBar`,
`getTextMessageString` in `ui_helpers.go` / `ui_render.go`) emits **tview color
tags** (`[red]`, `[-::d]`, `[::b]`) directly into `tview.TextView`. Bubbletea has
no such runtime tag parser — output must be plain text with ANSI escape codes.

**Options:**
- **A. Rewrite all print helpers to lipgloss styles.** Clean end state, but
  `PrintHelp()` alone builds ~40 formatted lines; helper strings also come from
  the `messages` package in a few places.
- **B. Write a small tview-tag → ANSI converter** (translate `[color]`, `[-]`,
  `[::b]`, `[::-]`, `[::d]` at render time). Preserves all call sites unmodified.
  Risk: incomplete subset handling (regions `["id"]` must be stripped, not
  translated).
- **C. Hybrid:** helpers become functions returning plain strings composed from
  lipgloss styles; a compat shim converts any remaining tagged strings. Regions
  syntax disappears entirely (see D2).

**Decision:** ✅ **A/C hybrid.** Convert every print helper to functions that
return plain strings styled with lipgloss (`PrintHelp`, `PrintError`,
`UpdateStatusBar`, `getTextMessageString`). No generic tag converter — a
half-compatible translator is worse than doing it properly. Region markers
(`["id"]`, `[""]`) are deleted outright in task 04 (see D2).

## D2. Message selection model (replacement for TextView regions)

**Problem:** Current UI uses tview *regions* + highlights to implement
"selected message": `ctx.CurRegions []messages.Message` + `TextView.Highlight()`
drive j/k navigation, `download`/`open`/`revoke`/`info` commands, and
`handleCopyUser`.

**Chosen approach:** Replace with an explicit model field
`selectedMsgIdx int` (-1 = none) over `CurRegions`. The viewport renders a
marker (reverse-video or a `>` gutter) on the selected line. This is simpler
than the region/ID-string indirection (`GetOffsetMsgId`) and removes the
`RenderingList`-style guards.

**Decision:** explicit index-based selection. ✅ settled in first assessment.

## D3. Widgets: bubbles vs custom rendering

**Options:**
- `bubbles/table`, `bubbles/viewport`, `bubbles/textinput` off the shelf.
- Custom rendered lists/viewports.

**Notes:**
- The three sidebar panes render differently (status/chats/groups share row
  format but distinct panes + focus ring); `bubbles/list` is heavier than
  needed; either `bubbles/table` or hand-rendered strings work. Hand-rendered
  likely simpler because `RenderChatTable` logic (pinned 📌, unread count in
  accent color, relative timestamp) is already string-building.
- `bubbles/viewport` is fine for the message pane but content sizing (wrapping
  long messages to terminal width) is our responsibility.
- `bubbles/textinput` fits the input line directly.

**Decision:** ✅ **Custom render for the three sidebar panes** (they are already
string-building with pinned/unread/timestamp columns; `bubbles/list` is too
heavy and `bubbles/table` fights the custom row format). **`bubbles/viewport`**
for the message pane, **`bubbles/textinput`** for the input line.

## D4. Mouse support policy

Current app has mouse enabled with a runtime toggle (Ctrl+p, `handleToggleMouse`).
Bubbletea's mouse support is coarser and **captures the terminal's native text
selection**, which matters for an IM client (users copy message text).

**Options:**
- A. Ship with mouse OFF by default, toggle ON via Ctrl+p (parity, safer default).
- B. Ship with mouse ON by default (parity with today, worse copy UX).
- C. Drop mouse support entirely.

**Decision:** ✅ **A — mouse OFF at startup, Ctrl+p toggles it on.** Preserves
terminal-native text selection by default (critical for an IM client), keeps
the existing toggle binding, and prints the same enable/disable notice as
`handleToggleMouse` does today.

**Options:**
- A. Keep everything in root `main` package, replacing files in place
  (`ui_*.go` rewritten).
- B. New `ui/` package with the tea model; root keeps only bootstrap.

**Notes:** `ui_helpers.go` print functions are called from root-package files
and reference `ctx.TextView` directly — a rewrite touches the same globals
either way. Root-file rewrite (A) keeps the diff reviewable file-by-file and
matches the existing "main = TUI layer" convention in CLAUDE.md.

**Decision:** ✅ **A — keep root `main` package**, replace `ui_*.go` in place.
Matches CLAUDE.md's stated architecture and keeps the diff reviewable per file.
Bubbletea model lives in `ui_model.go` (new) so `Update`/`View` logic has one
obvious home.

## D6. ANSI/emoji content in the message pane

`PrintQR` / `UpdateQR` (`ui_handler.go`) feed raw ANSI (via `tview.ANSIWriter`
/ `tview.TranslateANSI`) into the TextView. Bubbletea renders plain strings —
raw ANSI can be passed through verbatim in most terminals, but line widths and
wrapping of QR blocks need verification. `PrintImage` pipes an external command's
output (`ShowCommand`, usually chafa/viu) as ANSI into the pane.

**Decision:** ✅ Pass-through ANSI strings for QR/image output. The viewport
treats them as pre-rendered blocks: never re-wrap; compute the block's line
count from the source string, not from ANSI-aware measurement. Verify in task
04 acceptance checklist.

## D7. Keymap config compatibility

`config.Config.Keymap` values are tcell-style names today (e.g. "Ctrl+p"?
verify actual format strings in `config/settings.go` defaults). Bubbletea
matches against `tea.KeyMsg` with its own key names. We need a mapping layer
(parse config string → `tea.KeyMsg` predicate). Format must stay
backward-compatible with existing users' INI files.

**Decision:** ✅ Keep INI values unchanged (existing users' configs must keep
working). Confirmed default format in `config/settings.go`: modifiers spelled
`Ctrl+X` (e.g. `Ctrl+w`, `Ctrl+Space`, `Ctrl+?`), special keys spelled `Tab`,
and single characters bare (`d`, `i`, `o`, `u`, `r`, `s`). Parser handles exactly
those three shapes and errors loudly on anything else. Implemented in task 06.

## Version pins (task 01)

| Module | Version | Why |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | `v1.3.10` | Stable v1 line; v2 still beta |
| `github.com/charmbracelet/bubbles` | `v1.0.0` | viewport + textinput |
| `github.com/charmbracelet/lipgloss` | `v1.1.0` | styling, layout joins, wrapping |

Removed at the end of the migration (task 08): `rivo/tview`,
`gdamore/tcell/v2`, `code.rocketnine.space/tslocum/cbind`.
`atotto/clipboard`, `skratchdot/open-golang`, and `mattn/go-colorable` stay.
## Implementation log (tasks 02–08)

Decisions that were only implicit in the plan and how they were resolved in
code. Anything a reviewer would otherwise have to reverse-engineer belongs here.

### Structure

| File | Role |
|---|---|
| `ui_model.go` | `uiModel` (the `tea.Model`), `focusArea`, all UI message types, `Update`, `View`, command/quit wiring |
| `ui_keys.go` | `[keymap]` INI specs → bubbletea key strings, `action` enum, binding lookup |
| `ui_theme.go` + `ui_colors_gen.go` | `[colors]` INI values → lipgloss styles; generated X11 color-name table |
| `ui_layout.go` | pure string layout primitives (`fit`, rules, frame, `wrapText`) |
| `ui_render.go` | pure renderers (`formatRelativeTime`, chat rows, messages) |
| `ui_sidebar.go` | three panes with independent cursors, sort/split/infinite scroll |
| `ui_messages.go` | block-based message pane on `bubbles/viewport`, explicit selection |
| `ui_handler.go` | `UiMessageHandler` → `tea.Msg` translation only |
| `ui_helpers.go` | help screen, status bar, error/empty text (all return strings) |

`ui_keybindings.go` is gone: its `LoadShortcuts`/`EnterCommand`/
`ResetMsgSelection`/`GetOffsetMsgId` logic now lives in `ui_keys.go` +
`ui_model.go`, without cbind or regions.

### Behaviour notes

- **Focus model.** `focusArea` (status → chats → groups → messages → input)
  replaces `App.GetFocus()`. Config bindings are checked *before* widget
  handling, reproducing tview's app-level capture order (so `Ctrl+c`,
  `Ctrl+v`, `Ctrl+w` behave as before even while typing).
- **Message selection.** `messagesPane.selected` is an index into
  `entries` (D2). Falling off either end of the history matches the old
  `GetOffsetMsgId` fallback: moving *down* from "nothing selected" selects the
  oldest message, moving *up* selects the newest. Focusing the pane via the
  `focus_messages` binding selects the newest, like `handleFocusMessage`.
- **Top bar.** The old status update *replaced* the whole top bar text. Now the
  product name and the help hint stay on the left and the session status is
  right-aligned, so both remain visible. Content is otherwise identical.
- **Mouse.** Off at startup (D4); `Ctrl+p` toggles at runtime via
  `tea.EnableMouseCellMotion` / `tea.DisableMouse`, printing the same notice
  strings as `handleToggleMouse`. Clicking a sidebar row selects and opens that
  chat; clicking a message selects it; the wheel scrolls the message pane.
- **Layout.** Single outer frame with internal rules instead of a tview grid.
  The sidebar spans the full height (chat list keeps running past the input
  row), the rule above the input line and the input line itself stop at the
  sidebar separator, matching the old grid where the tables row-spanned.
- **Input line.** `bubbles/textinput` is focused at startup, Enter submits,
  Esc clears, and the row is painted with `[colors] input_background`. The
  widget's view is one cell wider than `Width`, so `Width = msgW-1` plus a
  defensive `ansi.Truncate`.
- **Re-flow on resize.** `messagesPane` stores un-wrapped *blocks* and derives
  display lines in `refresh()`, so a resize re-wraps history instead of leaving
  stale line breaks. ANSI passthrough blocks (QR, image output) are never
  wrapped (D6) and are covered by a unit test.
- **Errors.** Invalid `[keymap]` entries no longer print after the UI starts;
  they are collected by `buildKeyMap` and shown as the first line of the
  message pane: `invalid key binding: <field>: <reason>`.
- **Backend sends.** `sendCommand` returns a `tea.Cmd`, so a full
  `CommandChannel` can never stall the event loop; `queueCommand` (chat
  selection) does a non-blocking send with a goroutine fallback.
- **Colors.** `ui_colors_gen.go` was generated from `tcell.ColorNames` *before*
  task 08 removed the dependency; it is now a standalone 300-entry table. Any
  config value tcell accepted (hex, ANSI index, X11 name) still loads, and
  `colors.<field>: unknown color "…"` is reported at startup.
- **Notifications.** Untouched — `beeep` still fires from the backend, so
  desktop notifications keep working without the TUI being involved.
