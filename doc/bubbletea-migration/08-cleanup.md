# Task 08 — Remove tview/tcell/cbind, delete legacy files, fix tests

**Depends on:** 01–07 | **Blocks:** 09

## Goal
Zero legacy UI dependencies; file inventory matches new architecture.

## Steps
1. Delete or rewrite after all above done:
   - `main.go` (rewrite bootstrap), `app_context.go` (rewrite: no widget
     fields), `ui_layout.go` **delete**, `ui_keybindings.go` **delete**
     (logic moved to model update + key parser), `ui_handler.go` (rewritten
     in 07), `ui_helpers.go` (rewritten in 04/06), `ui_render.go` (rewritten
     in 03/04/06 — keep `formatRelativeTime` and its tests!).
2. `ui_render_test.go` — audit coverage: tests for `getTextMessageString`
   (region syntax) must be replaced by lipgloss-output equivalents; keep
   `formatRelativeTime` tests unchanged (pure function, no tview dep).
   If `ui_render_test.go` is main-package, ensure new tests land in package.
3. `go.mod`: remove `rivo/tview`, `gdamore/tcell/v2`,
   `code.rocketnine.space/tslocum/cbind`; `go mod tidy`.
4. CLAUDE.md project instructions: update Architecture table & pitfalls
   (UiMessageHandler=17 methods stays; tview→bubbletea wording; keymap/colors
   notes). Same for `agents.md`, `.cursorrules`, `GEMINI.md`, `SKILLS.md` if
   they name tview/tcell.
5. README: UI screenshots/notes updated (can defer final screenshot to 09).
6. CHANGELOG entry under Unreleased.

## Acceptance
- [ ] `grep -r "tview\|tcell\|cbind" --include="*.go"` → empty (and go.mod).
- [ ] Binary builds & runs; `make vet` clean; `go mod tidy` no-op.
- [ ] Docs no longer claim tview architecture.