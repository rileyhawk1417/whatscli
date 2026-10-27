# Task 09 — Test coverage, race verification, manual QA checklist

**Depends on:** 08 | **Gate: M4**

## Goal
Prove parity and stability: automated tests green under `-race`, manual QA
checklist executed end-to-end against real WhatsApp.

## Automated

`ui_model_test.go` (package `main`) covers everything that is pure logic or
plain model state, so most of the TUI is testable without a terminal:

| Area | Test | What it locks in |
|---|---|---|
| geometry | `TestViewGeometry`, `TestViewTooSmall` | exact width/height for 100x30, 80x24, 40x12, 200x60; placeholder below minimum |
| keymap | `TestCanonicalKey`, `TestBuildKeyMapDefaults`, `TestBuildKeyMapReportsInvalid` | all 17 bindings parse; pane-scoped `d`; invalid specs reported not ignored |
| keymap | `TestBackspaceEditsInputInsteadOfShowingHelp` | `Ctrl+?` and Backspace share a byte, so backspace must edit text in the input |
| input | `TestSubmitInputSendsCommand`, `TestSubmitInputPlainTextSendsMessage` | `/cmd` vs plain text routing, empty input is a no-op |
| messages | `TestNewScreenRendersMessages`, `TestMessageSelectionHighlight`, `TestEmptyScreenShowsHint` | history rendering, selection highlight, empty state |
| messages | `TestANSIPassthroughIsNotRewrapped`, `TestFreeFormOutputFollowsBottom` | QR/image blocks are never re-wrapped; help output follows the bottom unless the user scrolled up |
| sidebar | `TestSidebarSortAndSplit`, `TestSidebarSelectionSurvivesRefresh`, `TestSidebarRenderFillsHeight` | pinned-then-recent order, status/chat/group split, selection by JID, full-height panes |
| focus | `TestCycleFocusSkipsEmptyStatus`, `TestCtrlPReportsMouseState` | focus cycling and the mouse toggle notice |

Commands:

```bash
make test          # go test -race ./...   (messages/ included, untouched)
make vet
gofmt -l .         # messages/*.go were already unformatted before the migration
```

CI: `.github/workflows` runs build/vet/test; nothing extra to configure — the
UI tests need no terminal and no network.

## Terminal smoke test (no WhatsApp session needed)

`tools/pty_qa.py` starts the binary in a pseudo-terminal of a chosen size, sends
keystrokes, emulates the screen (cursor addressing, deferred wrap, scroll
regions) and prints the resulting frame. It answers lipgloss's OSC-11/DSR
queries, which a bare pty otherwise leaves hanging.

```bash
make build
./doc/bubbletea-migration/tools/pty_qa.py                       # startup screen
./doc/bubbletea-migration/tools/pty_qa.py --keys '/help\r'      # help + scroll-to-bottom
./doc/bubbletea-migration/tools/pty_qa.py --keys '\x10'         # Ctrl+p mouse notice
./doc/bubbletea-migration/tools/pty_qa.py --width 80 --height 24 --snapshot /tmp/frame.raw
```

Without a session the backend prints `connecting..` / `Please scan the QR code
with your phone`, so this verifies the frame, the input line, the help screen
and shutdown — not the chat flow.

## Manual QA checklist (record results here or in PR description)

Run against a real account (`whatscli --debug` for protocol logs):

- [ ] Fresh login: QR displays, scan completes, sidebar fills.
- [ ] Send/receive 1:1 chat; group chat; emoji/CJK names align.
- [ ] Unread counts update; read command clears them.
- [ ] Infinite scroll: >50 chats, reach bottom, +50 loads.
- [ ] Message selection nav + download/open/show/url/info/revoke/copyuser.
- [ ] Image render via ShowCommand inside pane; QR ANSI correct size.
- [ ] Backlog command pastes older messages; scroll-up rule holds.
- [ ] Mouse toggle Ctrl+p; native selection works when off (D4 policy).
- [ ] All INI keymap keys fire; INI colors theme the app (`/colorlist`).
- [ ] Status bar: online/offline, battery indicators (L/l, S/s).
- [ ] Quit → clean disconnect, `session_meta.db` intact, WAL sane.
- [ ] Terminal resize during all states (QR, chat view, help).
- [ ] Windows Terminal + mintty + tmux spot-checks (note failures by TERM).

### Results so far (Linux, xterm-256color, 110x30 pty)

| Check | Result |
|---|---|
| Startup frame, title bar, sidebar panes, input line | ✅ `pty_qa.py` |
| `/help` renders and scrolls into view; sections match the keymap | ✅ |
| Typing echoes in the input line; Esc clears | ✅ |
| `Ctrl+p` prints the state notice and toggles mouse reporting | ✅ |
| `Ctrl+q` disconnects, leaves the alt screen and restores the cursor | ✅ |
| Resize while running (SIGWINCH) | ✅ 110x30 → 80x24 frame stays rectangular |
| QR code renders from `UpdateQR` (21 block rows, ANSI passthrough) | ✅ `pty_qa.py` against a live login |
| Pairing failure surfaces as one error line, status stays offline | ✅ `--debug` run + `TestQRChannelTerminalEventReturnsError` |
| Image passthrough (`show` command) | ⚠️ needs a session with an attachment; covered by `TestANSIPassthroughIsNotRewrapped` |
| Everything requiring a WhatsApp session | ⚠️ pending a manual run |

## Login blockers found while smoke testing

Two problems made the first live run look like "connecting.. forever". Neither
was a rendering bug:

1. `whatsmeow` was pinned to a build that advertises web client version
   `2.3000.1033287297`; WhatsApp answers `<failure location="odn" reason="405"/>`
   and closes the pairing channel with `err-client-outdated` before any QR code
   is sent. Fixed by updating the dependency (`b601640`).
2. `loginWithQRCode` ignored those terminal events and then spun on the closed
   channel, so the failure never reached the UI. Fixed in `2b82022`, which also
   stops `runManager` from flipping the status bar back to "online" right after a
   rejected handshake.

The QR block is ~21 rows tall, so the login screen needs a terminal at least
~30 rows high (same limitation as the tview UI) — below that the pane clips it.

## Wrap-up
- [x] README/CLAUDE.md updated for the bubbletea stack.
- [x] Zero `tview` / `tcell` / `cbind` imports; dependencies removed.
- [ ] Live-account pass over the checklist above (M4 sign-off).
- [ ] Project memory checkpoint pointing at these docs.
