# Task 06 — Keybindings from config + color theme mapping

**Depends on:** 02–05, D7 (keymap compat) | **Blocks:** 07 | **Contributes to: M3**

## Goal
All shortcuts still come from `[keymap]` INI section and all 14 `[colors]`
keys style the UI — no hardcoded bindings except documented ones.

## Current state (what is replaced)
- `ui_keybindings.go → LoadShortcuts` — 14 global cbind bindings
  (FocusMessages, FocusInput, FocusChats, SwitchPanels, CommandRead,
  Copyuser, Pasteuser, CommandBacklog, CommandConnect "login", CommandQuit,
  CommandHelp, hardcoded Ctrl+p mouse toggle) + per-pane cbind sets attached
  via `SetInputCapture` on TextView/ChatTable/GroupTable/StatusTable.
- Config: `config/settings.go` Keymap + Colors structs (defaults at lines
  ~84/~106); INI `[keymap]`, `[colors]` sections.

## Steps
1. Key parsing layer (D7): parse INI key strings ("ctrl+p", "up", "Pg Up"?
   check defaults file) → `tea.KeyMsg` matchers. Write table-driven unit
   tests for the parser (first UI tests of the migration).
2. One central `Update` key dispatch: global keys work in every focus area;
   context keys (message-list keys, per-pane arrow handling from task 03)
   only in their focus area. Port each `handle*` function to a (model, msg)
   update rule; keep command-sending handlers in one place.
3. Colors: map the 14 `[colors]` keys (Borders, Background, Text, InputText,
   Positive, Negative, ChatMe, ChatContact, ForwardedText, ListContact,
   ListGroup, UnreadCount, …) → lipgloss color strings, resolved **once** at
   startup into a `theme` struct used by all render funcs. Replace all
   `tcell.ColorNames[...]` lookups.
   Note: `ShowColorList` iterates `tcell.ColorNames` — needs replacement
   strategy (lipgloss builtin names) — check with backend contract first.
4. Help text (`ui_helpers.go → PrintHelp`) updated to lipgloss styles + keys
   still read from config (D1); verify every listed key still fires.

## Acceptance
- [ ] Every `[keymap]` entry drives behavior in new UI; changing INI key
      changes the binding.
- [ ] All `[colors]` keys visibly affect the UI; invalid values error
      gracefully at startup (parity with current behavior).
- [ ] PrintHelp output matches old layout & content.
- [ ] Unit tests cover key parser + theme resolver (table-driven).

## Risks
- Key name format mismatch between tcell and config values — resolve exact
  default strings from `config/settings.go` before writing parser.