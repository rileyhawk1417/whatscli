# 10 — What remains, and how to do it

**Status:** planning dump (nothing here is started)
**Depends on:** tasks 01–09 (all ✅), theme system (`127b9f6`), painted surfaces (`1ab7c1a`)

This file is the pick-up-later plan for the bubbletea/themes work. Work happens
on branch `bubbletea-ui` (no upstream yet, HEAD = `1ab7c1a`); every item keeps
the standing agreements: `make build` + `go vet ./...` + `go test -race ./...`
green before commit, one commit per task, commitlint convention.

Current state to re-verify first:

```bash
git -C <repo> log --oneline -3        # expect 1ab7c1a, 127b9f6, eab6132
go vet ./... && go test -race -count=1 ./...
XDG_CONFIG_HOME=/tmp/xdg-q timeout 30 python3 doc/bubbletea-migration/tools/pty_qa.py --width 100 --height 30 --start-wait 5
```

The pty harness needs a fresh `XDG_CONFIG_HOME` (any empty dir) and answers
lipgloss's OSC-11/DSR probes; a QR login needs a terminal ≥ ~30 rows.

---

## A. M4 live-account sign-off — BLOCKING, needs a real WhatsApp account

The only migration gate not yet passed. **Never run a second instance against
the same account** — it gets kicked with `StreamReplaced`. Check the
`StreamReplaced` failure mode by comparing `./whatscli --debug` output if the
session drops unexpectedly.

Checklist (details in `09-testing-and-qa.md`):

- [ ] Login flow end to end: QR renders, pairing succeeds, `session_meta.db`
      and `session.db` written, next launch skips login.
- [ ] Real chat traffic: 1:1 send/receive, group chat, emoji/CJK names align,
      unread counts appear and clear with `/read`.
- [ ] Contact names resolve in the sidebar within a couple of seconds of
      connecting (the `a0c4a55` fix — history, message pane and `/search`
      should all show names, not numbers).
- [ ] Infinite scroll: >50 chats, reach pane bottom, +50 loads.
- [ ] Media: message selection nav + `download` / `open` / `show` / `url` /
      `info` / `revoke` / `copyuser`; image passthrough inside the pane.
- [ ] `/backlog` pastes older messages; scroll-up rule holds.
- [ ] `Ctrl+p` mouse toggle; native text selection when off.
- [ ] Theme spot-checks **(new since `eab6132`/`1ab7c1a`)**:
      - `/theme` opens the picker, arrow keys repaint the whole surface live,
        Enter keeps, Esc restores the previous palette.
      - `theme = light` on a dark terminal is legible (painted white surface);
        QR renders correctly on it (paint pass re-arms between QR spans).
      - `theme = classic` prints the auto-contrast notice and lifts the dark
        tones; `theme = mono` shows no color at all.
- [ ] Quit (`Ctrl+q`) disconnects cleanly; WAL intact after restart.
- [ ] Resize during QR, chat view and help states.
- [ ] tmux + Windows Terminal + mintty spot-checks (note failures by `$TERM`).

Then fold results into the M4 table in `09-testing-and-qa.md` and tick the
"Live-account pass" box.

## B. Ship the branch

1. `git push -u origin bubbletea-ui` — the branch has never been pushed.
2. Open the PR against `master` (description can crib from CHANGELOG's
   "Unreleased" section + `INDEX.md`).
3. After review/merge: consider `release.sh` for a new version — read the full
   flow first, it updates a separate Homebrew tap repo.
4. Optionally regenerate `doc/screenshot.png`: the current one is v0.6.5 and
   predates even the tview UI. Best done with a live session so the sidebar has
   real chats; consider two shots (dark default + `theme = light`) since the
   surface is painted now.

## C. User-decision items (each is a small task if accepted)

| # | Decision | How to do it if accepted |
|---|----------|--------------------------|
| C1 | Replace the old loud palette: delete the whole `[colors]` section from `~/.config/whatscli/whatscli.config` so the new readable defaults apply | Back the file up first (`cp whatscli.config whatscli.config.bak`); deleting the section is all that is needed — absent keys fall back to built-in defaults (`theme = dark` included) |
| C2 | Sidebar timestamp truncation at 30 cells (`Sep 13, 20`) | Either bump default `chat_sidebar_width` to ~34 or drop the year from `formatRelativeTime`; the latter keeps narrow terminals useful |
| C3 | Retro-rename of the *open* chat after contact data arrives | Message pane reads from SQLite on selection only; to fix, trigger a pane refresh in `applyContactNames`/`renameConversation` when the renamed JID is the currently open chat |

## D. Theme feature follow-ups (optional, in rough priority order)

1. **`/theme save`** — persist the current preset to disk. Write-back via
   `gopkg.in/ini.v1` on `GetConfigFilePath()`. Gotchas:
   - Write color *names*, never bare hex (`#` starts a comment); if a quoted
     hex must be preserved, keep the user's quoting (`resolveColor` strips
     `"`, `'`, backticks, so round-tripping is possible by detecting them in
     the raw key value).
   - Only write the `theme` key plus keys the user explicitly customized
     (`config.ColorKeysSet`); do not serialize the whole palette, or the file
     stops tracking future preset improvements.
   - Make a `.bak` copy before the first write-back.
2. **Custom themes from a file** — `theme = mytheme` resolving to
   `~/.config/whatscli/themes/mytheme.ini` with the same 14 `[colors]` keys.
   Loading path: extend `findTheme` to try preset names first, then fall back
   to an INI file parsed with `loadTheme(name-from-file)`; `themeChoices`
   lists found files. The paint/contrast machinery needs no changes — it works
   on any palette.
3. **OSC 11 background detection** — lipgloss already probes at startup
   (that's the `\x1b]11;?` handshake the pty harness answers). Could pick
   `light` vs `dark` automatically for installs with *no* config file. Note:
   with painted surfaces (D10) legibility no longer depends on it; it's only
   about which *factory* palette a brand-new install starts on. Low value
   while `theme = dark` is the documented default — skip unless asked.
4. **Theme preview in the README** — ANSI screenshot block; depends on B4.

## E. Housekeeping

- Consolidate `.pi/memory` project categories (`projects/whatscli` vs
  `projects/whatscli` overlap flagged by the index) — via the memory tool, not
  file edits.
- `gofmt -l .` still flags three pre-existing files
  (`messages/cmd_registry.go`, `messages/history_sync.go`,
  `messages/messages.go`) — documented in task 09; fix in one tiny commit if
  desired, untouched otherwise.

## F. Bugs

| # | Bug | Status |
|---|-----|--------|
| B1 | **Cannot type or navigate on the QR/login screen.** User report (2026-10-27): input untypeable, focus jumping. Root cause: the QR countdown calls `UpdateQR` every second and `showQR` treated every tick as a full screen rebuild, adding `m.focus = focusInput` + `m.input.SetValue("")` — the pre-migration `UpdateQR` only rewrote the text area. Fixed in the same session; rules in the "Post-milestone fix" note in `00-design-decisions.md`, regression test `TestShowQRCountdownKeepsInput` | ✅ fixed |

## Suggested order when picking this up

```
C1 (one minute, user consent) → A (M4 sign-off, live account) → B (push/PR)
→ C2/C3 → D1 (/theme save) → D2 (custom themes) → B4 (screenshot) → E
```

Nothing in C/D blocks anything else; A is the only true gate.