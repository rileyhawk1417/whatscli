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