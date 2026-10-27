# Task 09 — Test coverage, race verification, manual QA checklist

**Depends on:** 08 | **Gate: M4**

## Goal
Prove parity and stability: automated tests green under `-race`, manual QA
checklist executed end-to-end against real WhatsApp.

## Automated
1. Unit tests (table-driven, `package main`, in-memory where applicable):
   - keymap parser (from 06) — done
   - theme resolver — done
   - chat list model: sort order, pinned, infinite scroll trigger
   - message rendering: sender coloring, forwarded, skip-empty, wrapping
   - selection navigation: j/k ±1, PgUp/PgDn ±10, g/G bounds, Esc clear,
     GetOffsetMsgId-port edge cases (empty list, out-of-range offset)
   - input: prefix detection, params split, empty receiver
   - UpdateQR/port content composition
2. `make test` with `-race` — full suite, including all `messages/` tests.
3. `make vet`, golangci-lint if configured in repo CI (check `.github/`).

## Manual QA checklist (record results here or in PR description)
- [ ] Fresh login: QR displays, scan completes, sidebar fills.
- [ ] Send/receive 1:1 chat; group chat; emoji/CJK names align.
- [ ] Unread counts update; read command clears them.
- [ ] Infinite scroll: >50 chats, reach bottom, +50 loads.
- [ ] Message selection nav + download/open/show/url/info/revoke/copyuser.
- [ ] Image render via ShowCommand inside pane; QR ANSI correct size.
- [ ] Backlog command pastes older messages; scroll-up rule holds.
- [ ] Mouse toggle Ctrl+p; native selection works when off (D4 policy).
- [ ] All INI keymap keys fire; INI colors theme the app.
- [ ] Status bar: online/offline, battery indicators (L/l, S/s).
- [ ] Quit → clean disconnect, `session_meta.db` intact, WAL sane.
- [ ] Terminal resize during all states (QR, chat view, help).
- [ ] Windows Terminal + mintty + tmux spot-checks (note failures by TERM).

## Wrap-up
- [ ] README/CHANGELOG final; M4 gate met; merge PR to main.
- [ ] Drop a project memory checkpoint pointing at these docs.