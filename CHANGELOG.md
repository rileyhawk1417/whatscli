# Changelog

All notable changes to the WhatsApp CLI application will be documented in this file.

## [Unreleased]

### Fixed
- Chat lists name people instead of showing bare phone numbers. Names come from
  the address book, then the contact's own push name, then a verified business
  name; chats created before WhatsApp delivered that data are relabelled too
  (including the sender column of already stored messages), so the message pane
  and `/search` show names as well.
- QR login works again: `whatsmeow` now advertises a current web client version,
  so WhatsApp no longer rejects the handshake with a 405 before sending a code.
- Pairing failures are reported instead of looping silently on a closed channel,
  and a rejected handshake no longer flips the status bar to "online".

### Changed
- The TUI now runs on [Bubbletea](https://github.com/charmbracelet/bubbletea),
  [Bubbles](https://github.com/charmbracelet/bubbles) and
  [Lip Gloss](https://github.com/charmbracelet/lipgloss); `tview`, `tcell` and
  `cbind` are gone.
- Existing INI configuration keeps working unchanged: `[keymap]` values and all
  `[colors]` keys (hex, ANSI index or X11/CSS name) load as before.
- Mouse reporting is now off at startup so the terminal's native text selection
  works; `Ctrl+p` still toggles it.
- The status bar keeps the product name and help hint on the left and shows the
  session status on the right.
- Invalid `[keymap]` entries are reported as a message instead of being ignored.
- New installs get a readable default palette (soft white text, amber headers,
  muted blue senders, grey borders) instead of terminal-basic `blue`/`purple`,
  which sat at 2.4:1 and 2.2:1 contrast on a black background.

### Added
- Built-in colour themes: `theme` in the `[colors]` section selects `dark`
  (default), `light`, `classic` or `mono`, and individual colour keys still
  override the preset. `/theme` opens a selector that previews each palette on
  the whole UI, `Enter` applies it for the session and `Esc` restores the
  previous one.
- `auto_contrast` in the `[colors]` section (on by default): foreground colors
  that would fall below the 4.5:1 WCAG contrast ratio against `background` are
  lightened along their own hue instead of rendering nearly invisible. Legacy
  settings are affected most — `blue` (2.4:1) becomes a readable blue — and a
  one-line note in the message pane lists what was raised.
- Timestamps and hints use a `dim` tone derived from `text`/`background` rather
  than a fixed dark grey, so they read as secondary without disappearing.
- `doc/bubbletea-migration/` — the migration plan, design decisions and a
  terminal smoke-test harness (`tools/pty_qa.py`).
- Headless tests for the UI model: frame geometry, keymap parsing, input
  routing, message selection, sidebar sorting and ANSI pass-through.

## [2023-03-17]

### Changed
- Migrated from deprecated `go-whatsapp` library to the newer `whatsmeow` library
- Complete rewrite of the session management system for better reliability
- Enhanced message handling with improved error handling and feedback
- Updated UI interactions to work with the new backend

### Added
- Automatic loading of recent chats after successful login or session restoration
- Loading of the 20 most recent messages for each chat, including text and image messages with captions
- Loading of WhatsApp contacts from the address book to ensure proper name display
- Improved backlog command with multiple strategies for retrieving message history:
  - Read receipt trigger method
  - Chat presence (typing status) trigger method
  - Direct history sync notification method
- Better error handling with user-friendly messages
- More robust session restoration and QR code login process

### Fixed
- Nil pointer dereference errors in message history retrieval
- Type mismatches in the backlog command implementation
- Issues with contact name display in messages and chat list
- Improved session reconnection after disconnect

### Security
- Updated to the latest WhatsApp protocol implementation
- Better handling of encrypted sessions 
