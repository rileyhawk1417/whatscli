# INDEX — tview → Bubbletea Migration Tasks

Single source of truth for the migration's task list. Keep this file **lean**:
one line per task, status, and a link to the detail file. Details, code
touchpoints, and acceptance criteria live in the task files.

Branch: `bubbletea-ui` (created from `master`)
Design decisions: `00-design-decisions.md` (D1–D7, all settled)

## Phases

### Phase 1 — Foundation
| ID | Task | Depends on | Status | Detail |
|----|------|-----------|--------|--------|
| 01 | Dependencies, branch, and project skeleton | — | ✅ done | [01-dependencies-and-skeleton.md](01-dependencies-and-skeleton.md) |
| 02 | App model, Update/View skeleton, program lifecycle | 01, D3, D5 | ✅ done | [02-app-model-skeleton.md](02-app-model-skeleton.md) |

### Phase 2 — Panels
| ID | Task | Depends on | Status | Detail |
|----|------|-----------|--------|--------|
| 03 | Sidebar: status / chats / groups lists | 02, D3 | ✅ done | [03-sidebar-lists.md](03-sidebar-lists.md) |
| 04 | Message pane: viewport, rendering, selection | 02, D1, D2, D6 | ✅ done | [04-message-pane.md](04-message-pane.md) |
| 05 | Input line and command entry | 02 | ✅ done | [05-input-line.md](05-input-line.md) |

### Phase 3 — Behavior parity
| ID | Task | Depends on | Status | Detail |
|----|------|-----------|--------|--------|
| 06 | Keybindings from config + color theme mapping | 02–05, D1, D7 | ✅ done | [06-keybindings-and-theme.md](06-keybindings-and-theme.md) |
| 07 | UiHandler rewiring to program.Send + state sync | 02 | ✅ done | [07-uihandler-wiring.md](07-uihandler-wiring.md) |

### Phase 4 — Cleanup & verification
| ID | Task | Depends on | Status | Detail |
|----|------|-----------|--------|--------|
| 08 | Remove tview/tcell/cbind, delete legacy files, fix tests | 01–07 | ✅ done | [08-cleanup.md](08-cleanup.md) |
| 09 | Test coverage, race verification, manual QA checklist | 08 | ◐ in progress | [09-testing-and-qa.md](09-testing-and-qa.md) |

## Working agreements

- Every task: build green (`make build`), `make vet` clean, `make test` passing
  before commit; one commit per task, commitlint convention respected.
- `messages/` package is **frozen** during this migration (interface-compatible
  only); backend tests must pass untouched.
- Any discovered behavior change goes through a decision entry in
  `00-design-decisions.md` before it's implemented.
- Status legend: ☐ not started / ◐ in progress / ✅ done / ⊘ dropped.

## Hard milestones (acceptance gates)

- M1 (after 02): bubbletea binary renders static skeleton in terminal, Ctrl+c quits.
- M2 (after 05): full read/write chat flow works against real WhatsApp login.
- M3 (after 07): feature parity vs `doc/screenshot.png` — QR login, chat lists,
  message selection, keymap-driven shortcuts, mouse toggle, status bar.
- M4 (after 09): green `-race` CI, zero tcell/tview imports, README updated.