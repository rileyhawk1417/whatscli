# Task 04 — Message pane: viewport, rendering, selection

**Depends on:** 02, D1 (markup), D2 (selection model), D6 (ANSI passthrough)
**Blocks:** 07 | **Contributes to: M2, M3** — hardest task in the migration.

## Goal
Faithful port of the scrolling message viewport: message rendering with
sender/time coloring, per-message selection with j/k/g/G navigation, QR and
image output passthrough.

## Current state (what is replaced)
- `ui_handler.go → NewMessage / NewScreen / PrintQR / UpdateQR` — text/ANSI
  pushed into TextView; `CurRegions = msgs` bookkeeping.
- `ui_render.go → getMessagesString / getTextMessageString` — time format
  `02-01-06 15:04:05`, `[colorMe/Contact::b]` sender coloring, forwarded
  tint, `tview.Escape` (drops!), tview region markers `["id"]`/`[""]`.
- `ui_keybindings.go → handleMessagesMove / handleMessagesFirst /
  handleMessagesLast / handleExitMessages / GetOffsetMsgId /
  ResetMsgSelection` — region-based selection navigation.
- `ui_render.go → PrintImage` — runs `config.General.ShowCommand`, pipes
  stdout ANSI into `tview.ANSIWriter` (D6).

## Steps
1. Port `getTextMessageString/getMessagesString` to lipgloss (D1): same
   fields — dim timestamp, bold colored sender, forwarded color; strip region
   markers entirely (D2).
2. `bubbles/viewport` for scroll; wrap long lines to viewport width
   **before** adding to content (own wrapping — lipgloss `Width` +
   `wordwrap`); auto-follow-to-bottom on new message unless user scrolled up
   (parity: tview `ScrollToEnd` behavior on print).
3. Selection (D2): `selectedMsgIdx int`; render selected message reversed or
   with a gutter marker. Navigation: j/k/↑/↓ = ±1, PgUp/PgDn/Ctrl+u/Ctrl+d =
   ±10, g = first, G = last, Esc = clear (exact port of
   `keysMessages` bindings in `ui_keybindings.go`).
4. Selected message id must remain resolvable for backend commands
   (`download`, `open`, `show`, `url`, `info`, `revoke`, `copyuser` pass
   `CurRegions[idx].Id` — see `handleMessageCommand`/`handleCopyUser`).
   Keep `CurRegions` as the source of truth for ids.
5. QR: `PrintQR`/`UpdateQR` — port `UpdateQR` output composition
   (header + attempt/timeout + help) and `tview.TranslateANSI` → raw ANSI
   passthrough per D6; auto-scroll fully down; refocus input.
6. Images: `PrintImage` port including ALL security checks (path clean,
   symlink eval, download/preview dir prefix — audit S-2 regression risk).

## Acceptance
- [ ] Long messages wrap at pane width; no visual corruption.
- [ ] j/k/g/G/PgUp/PgDn/Esc selection behaves identically to today.
- [ ] Selecting message + `MessageInfo` etc. sends correct message id.
- [ ] QR login screen renders scannable; auto-refresh text shows.
- [ ] `ShowCommand` image output renders (chafa/viu) with correct size.
- [ ] Message pane auto-scrolls to bottom on new message when at bottom.

## Risks
- ANSI measurement: viewport must not re-measure pre-wrapped ANSI blocks —
  embed content with explicit line count (validate in task 09 QA).
- Newlines inside message text and empty protocol messages (skipped in
  `getMessagesString`) — preserve skip behavior.