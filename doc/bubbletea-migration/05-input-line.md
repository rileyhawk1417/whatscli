# Task 05 — Input line and command entry

**Depends on:** 02 | **Blocks:** 06 | **Gate: M2**

## Goal
Port the command input line: command-prefix parsing, send fallback, paste
integration, Esc clear.

## Current state (what is replaced)
- `ui_keybindings.go → EnterCommand` (wired as `InputField.SetDoneFunc`):
  - empty input → no-op
  - starts with `config.General.CmdPrefix` → `Command{Name, Params}` via
    `strings.Fields` + prefix trim
  - otherwise → `send` to `CurrentReceiver.Id`; empty receiver →
    "no receiver" print
- `handlePasteUser` — clipboard append into input with leading space.
- Esc clears input (`tcell.KeyEsc` branch).

## Steps
1. `bubbles/textinput` styled with `Colors.InputText` (task 06 styles it).
2. Port `EnterCommand` logic into Update on Enter KeyMsg — exact semantics,
   including empty-input no-op and `no receiver` message.
3. Key handling when focused: Esc clears; typing is textinput default.
   Tab from messages pane refocuses input (`handleMessagePanelTab`).
4. Copy/paste: `clipboard.ReadAll` on Pasteuser key appends with space;
   keep `atotto/clipboard` dep.

## Acceptance
- [ ] `CmdPrefix command args…` dispatches to backend command channel.
- [ ] Plain text sends to current receiver; no receiver → "no receiver".
- [ ] Esc clears input; input survives list re-renders (state stays in model).
- [ ] Paste appends correctly with cursor at end.

## Risks
- Cursor/scroll behavior of textinput in a 1-line bottom row — cosmetic
  check in QA.