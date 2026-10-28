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

## D8. Readable palettes (contrast floor)

The `[colors]` defaults were written for tcell, where `blue` means the *dark*
ANSI tone. On a black background `#0000ff` is 2.4:1 and `#800080` is 2.2:1 —
fine for a 1980s terminal, unreadable for sender names and forwarded text in a
full-screen UI. Two changes, both backwards compatible:

1. **`auto_contrast` (new key, default `true`).** Every foreground color is
   measured against the surface it is drawn on (main `background`, or
   `input_background` for `input_text`) using the WCAG relative-luminance
   formula. Anything under 4.5:1 (3:1 for `borders`, which is a graphical
   object rather than text) is blended towards white — or towards black when
   the background is light — until it clears the floor. The hue is preserved, so
   `blue` becomes `#6060ff` rather than a different color. Adjustments are
   collected in `theme.Adjusted` and surfaced as one line at startup, so a
   changed palette is never silent. `auto_contrast = false` restores the exact
   configured values.
2. **`dim` is derived, not hard-coded.** Timestamps and hints used a fixed
   `#696969` (3.8:1, effectively invisible at the end of a row). They are now
   `text` mixed 45% towards `background`, then passed through the same contrast
   floor — secondary, but never below 4.5:1.

The shipped defaults were replaced with a readable palette (soft white text,
amber headers, muted blue senders, grey borders). Existing configs keep their
values; only the unreadable ones are lifted. Code lives in `ui_contrast.go`
(math + xterm→RGB) and `theme.applyContrast` (`ui_theme.go`).

## D9. Themes: presets plus overrides

Hard-coded palette constants were the last thing keeping the migration from
being configurable end to end, so the `[colors]` section gained a `theme` key
backed by four built-in palettes (`ui_themes.go`): `dark` (the shipped
defaults), `light` (dark-on-white), `classic` (the pre-1.3 ANSI palette, which
the contrast floor lifts) and `mono` (greyscale). The rules:

1. **A preset fills, the file wins.** `loadThemeWith` resolves the preset and
   then layers the keys the config file *actually contains* over it —
   `config.ColorKeysSet` records that at parse time, because `MapTo` cannot
   distinguish "absent" from "set to the default". `theme = mono` plus
   `chat_me = red` therefore means a greyscale UI with red for your own
   messages. Because of the same rule an old config that still holds all
   fourteen keys keeps its exact palette even though `theme` defaults to
   `dark`.
2. **The picker is a session-only change.** `/theme` (with no argument) opens
   an overlay listing `custom` + the presets; arrow keys re-render the entire
   UI — including already-scrolled message history and free-form output, via
   `messagesPane.retint` / `uiModel.retint` — while `Enter` keeps the choice
   and `Esc` restores the palette that was active before. Nothing is written to
   disk, and ANSI pass-through blocks (QR codes, images) are never re-tinted,
   so a preview cannot corrupt them.
3. **`custom` means "exactly what startup produced".** It re-resolves
   `[colors]` the same way `main` did, which makes the row the baseline to
   compare a preset against and the escape hatch after an abort.
4. **Presets still obey `auto_contrast`.** The user's setting is carried across
   every switch, and the shipped presets are chosen so that the floor has
   nothing to lift. The `classic` preset exists precisely so the lifted legacy
   palette is still available under a name.

Known limitation, documented in the README: whatscli does not paint its own
background, so the frame and message area show the terminal's. The `light`
preset therefore assumes a light terminal, which is also why the preset is
described rather than auto-detected — `theme` stays explicit so a user on a
light terminal never gets dark-on-dark after an upgrade.

## D10. Painted surfaces

The migration originally left backgrounds to the terminal, which made the
`light` preset useless on a dark terminal and meant `background`/`input_background`
only showed through behind the input field. Now that every cell of the frame is
produced by our own compositor, the paint pass (`ui_background.go`, applied at
the end of `View`) covers the whole surface instead:

1. Every frame line is re-armed with the `background` SGR sequence: the
   sequence prefixes the line, is re-inserted after every `\x1b[0m`, and short
   lines are padded with painted spaces. Without this, *any* styled span would
   drop back to the terminal background after its own reset — lipgloss emits a
   reset per segment and ANSI pass-through blocks (QR, images) carry their own
   spans, so per-line wrapping in a `Background` style cannot work.
2. The input row is painted with `input_background` instead, preserving the
   field look from the tview version.
3. The pass runs after the theme-picker overlay also covers the picker rows.
4. The sequences are built through lipgloss (`backgroundSequence`), so the
   active color profile picks the encoding (truecolor, 256 or basic ANSI), and
   a profile without colors yields "" — the pass then degrades to the exact
   pre-migration behaviour instead of emitting garbage sequences.
5. `retint` still leaves ANSI pass-through blocks alone: the paint pass never
   rewrites their *content*, it only re-arms the surface between their own
   color spans, which is what makes QR codes render correctly on a `light`
   theme (black modules stay black, the quiet zone is painted from the theme).

Consequence: `background` is now what the user sees, full stop. The README no
longer tells people to keep the terminal's background in sync.

## D11. Editor-palette presets (ports)

`catppuccin`, `tokyonight` and `solarized` are ports of popular editor
colourschemes rather than palettes invented for the TUI. Three rules keep the
ports honest:

1. **Use upstream values, do not approximate them.** Every key comes from the
   scheme's own palette (Catppuccin Macchiato, Tokyo Night Storm, Solarized
   Osaka dark). The source files live in `theme-refs/` so a refresh can be
   diffed against upstream later.
2. **Map by meaning, not by editor syntax role.** Editor schemes are organised
   around `Keyword`/`String`/`Comment`; the TUI needs semantic roles. The
   mapping is: `list_header`/`unread_count` → the scheme's warm accent,
   `list_contact`/`positive` → its green, `list_group`/`chat_me` → its blue,
   `chat_contact` → its teal/cyan, `forwarded_text` → its violet/magenta,
   `negative` → its red, `borders` → the faintest tone that still clears the
   3:1 graphical floor, `input_background` → a raised surface one step off the
   background.
3. **Pick a brighter step instead of relying on auto_contrast.** A preset must
   render as written, so a colour that fails the floor is replaced by the next
   step of the same ramp (Osaka's `magenta` `#d23681` at 4.12:1 becomes
   `#f254a1` at 5.89:1). This is what `TestPresetColorsClearContrastFloor`
   enforces for every preset except `classic`, which exists precisely to make
   the lifted legacy palette available under a name.

Consequence for the selector: preset names no longer fit in the old 9-cell
name column, so `themePicker.render` widened it (and the box) to 11 cells / 50
inner columns. `themeNames()` still preserves declaration order with `mono`
last, which is what the picker clamp test asserts.

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

## Post-milestone fix: QR countdown must not own the input line

**Bug (reported live, 2026-10-27):** on the QR/login screen you cannot type —
everything typed is erased within a second, focus jumps back to the input
constantly, and any help/error output printed next to the QR is wiped each
tick. Root cause: `handleQRChannel` ticks once per second for the countdown
and calls `UpdateQR` every tick; the ported `showQR` treated every tick as a
screen rebuild *and* did `m.focus = focusInput` + `m.input.SetValue("")`.
The pre-migration `UiHandler.UpdateQR` cleared and reprinted the TView *text
area* only — the input widget was never touched, so typing survived. The
`SetValue("" )`/focus-forcing lines in the port were therefore a deviation.

**Fix rules (settled):**

1. `showQR` never mutates `m.input` or `m.focus`, in any branch. The model
   starts with the input focused, which is the login screen's natural state.
2. A full rebuild (`clear()` + help + header + QR) happens only when the QR
   payload or its attempt number changed — the moments the old handler also
   logically replaced the screen.
3. A same-QR countdown tick updates only the countdown header line in place
   (index recorded at build time; falls back to a rebuild if the pane was
   reorganized). This keeps user-printed output (help, errors) on the login
   screen alive while the countdown still runs — a deliberate small
   improvement over the old wipe-per-tick, justified because the new pane is
   one shared stream instead of a dedicated widget.
4. Scrolling on the login screen is no longer yanked to the bottom every
   second; `GotoBottom` happens only when the screen actually rebuilds.
