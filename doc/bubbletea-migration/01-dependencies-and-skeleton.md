# Task 01 — Dependencies, branch, and project skeleton

**Depends on:** — | **Decisions needed:** none (informs D3, D5)
**Blocks:** 02

## Goal
Create the migration branch, add the bubbletea dependency stack, and prove the
toolchain builds — without changing any behavior.

## Steps
1. Create branch `ui/bubbletea-migration` off `main`.
2. `go get github.com/charmbracelet/bubbletea github.com/charmbracelet/bubbles github.com/charmbracelet/lipgloss` (pin versions in `go.mod`).
3. Note: `go.mod` currently has `gdamore/tcell/v2 v2.13.8`, `rivo/tview v0.42.0`,
   `code.rocketnine.space/tslocum/cbind`, `atotto/clipboard`,
   `skratchdot/open-golang`. **Remove `cbind` in this task or in 08** — record
   choice here. Clipboard and open stay (used by handlers we keep).
4. CGO still required (sqlite3) — verify `make build` unaffected by new deps.
5. Add a trivial `hello.go` spike or a scratch `//go:build ignore` file that
   starts a minimal `tea.Program` and exits, just to verify terminal I/O works,
   then delete it (or keep in a scratch branch).

## Acceptance
- [ ] `make build && make vet && make test` green on branch.
- [ ] go.mod contains bubbletea/bubbles/lipgloss; `go mod tidy` leaves no
      unused imports.
- [ ] Minimal tea program runs and quits cleanly in a real terminal.

## Risks
- Terminal compatibility of bubbletea on user's TERM (Windows/old terminals) —
  note outcome in README later (task 09).