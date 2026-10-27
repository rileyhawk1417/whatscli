package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/normen/whatscli/config"
)

// action is a UI-level operation triggered by a key binding.
type action int

const (
	actNone action = iota
	actFocusMessages
	actFocusInput
	actFocusChats
	actSwitchPanels
	actCopyUser
	actPasteUser
	actCommandRead
	actCommandBacklog
	actCommandConnect
	actCommandQuit
	actCommandHelp
	actToggleMouse
	actMessageDownload
	actMessageOpen
	actMessageShow
	actMessageUrl
	actMessageInfo
	actMessageRevoke
)

// keyMap resolves a config key spec to an action. Specs are stored in the same
// spirit as the old cbind configuration: global bindings plus bindings that
// only fire while the message pane has focus.
type keyMap struct {
	global   map[string]action
	messages map[string]action
	// backspaceIsHelp records that some binding parsed to "backspace" (the
	// terminal sends the same byte for Ctrl+? and Backspace). While the input
	// line is focused that key must edit text instead of firing the action.
	backspaceIsHelp bool
}

// bindingSpec pairs a config value with the action it triggers.
type bindingSpec struct {
	name string
	spec string
	act  action
	msg  bool // message-pane scoped
}

// buildKeyMap parses the [keymap] section. Every invalid spec is reported so
// users see exactly which INI entry needs fixing, matching the old
// PrintErrorMsg behaviour on cbind.Set failure.
func buildKeyMap(km config.Keymap) (keyMap, error) {
	specs := []bindingSpec{
		{"switch_panels", km.SwitchPanels, actSwitchPanels, false},
		{"focus_messages", km.FocusMessages, actFocusMessages, false},
		{"focus_input", km.FocusInput, actFocusInput, false},
		{"focus_contacts", km.FocusChats, actFocusChats, false},
		{"copyuser", km.Copyuser, actCopyUser, false},
		{"pasteuser", km.Pasteuser, actPasteUser, false},
		{"command_read", km.CommandRead, actCommandRead, false},
		{"command_backlog", km.CommandBacklog, actCommandBacklog, false},
		{"command_connect", km.CommandConnect, actCommandConnect, false},
		{"command_quit", km.CommandQuit, actCommandQuit, false},
		{"command_help", km.CommandHelp, actCommandHelp, false},
		{"message_download", km.MessageDownload, actMessageDownload, true},
		{"message_open", km.MessageOpen, actMessageOpen, true},
		{"message_show", km.MessageShow, actMessageShow, true},
		{"message_url", km.MessageUrl, actMessageUrl, true},
		{"message_info", km.MessageInfo, actMessageInfo, true},
		{"message_revoke", km.MessageRevoke, actMessageRevoke, true},
	}

	out := keyMap{global: map[string]action{}, messages: map[string]action{}}
	var errs []string
	for _, s := range specs {
		key, err := canonicalKey(s.spec)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", s.name, err))
			continue
		}
		if s.msg {
			out.messages[key] = s.act
		} else {
			out.global[key] = s.act
		}
	}
	if _, ok := out.global["backspace"]; ok {
		out.backspaceIsHelp = true
	}
	// The mouse toggle has always been hardcoded as Ctrl+p.
	out.global["ctrl+p"] = actToggleMouse

	if len(errs) > 0 {
		return out, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

// lookup returns the action bound to a key string, preferring global bindings
// (the old app-level input capture ran before widget handling) and falling back
// to message-pane bindings when that pane is focused.
func (k keyMap) lookup(key string, area focusArea) action {
	if key == "backspace" && area == focusInput && k.backspaceIsHelp {
		return actNone
	}
	if a, ok := k.global[key]; ok {
		return a
	}
	if area == focusMessages {
		if a, ok := k.messages[key]; ok {
			return a
		}
	}
	return actNone
}

// canonicalKey translates an INI key spec into the string bubbletea reports for
// that key (tea.KeyMsg.String()). Three shapes are supported, which covers
// every default and every documented value:
//
//	Ctrl+X        → "ctrl+x"   (X may be a letter or one of \ ] ^ _)
//	Ctrl+Space    → "ctrl+@"   (NUL)
//	Ctrl+?        → "backspace" (DEL, which is what terminals send)
//	Tab / Esc / … → "tab", "esc", …
//	d / G / ?     → "d", "G", "?"
func canonicalKey(spec string) (string, error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return "", fmt.Errorf("empty key binding")
	}
	lower := strings.ToLower(s)

	if strings.HasPrefix(lower, "ctrl+") {
		rest := s[len("ctrl+"):]
		switch strings.ToLower(rest) {
		case "space":
			return "ctrl+@", nil
		case "?":
			return "backspace", nil
		}
		if utf8.RuneCountInString(rest) == 1 {
			r := []rune(strings.ToLower(rest))[0]
			if (r >= 'a' && r <= 'z') || r == '\\' || r == ']' || r == '^' || r == '_' {
				return "ctrl+" + string(r), nil
			}
		}
		return "", fmt.Errorf("unsupported key %q (use Ctrl+A-Z, Ctrl+Space or Ctrl+?)", spec)
	}

	switch lower {
	case "tab":
		return "tab", nil
	case "shift+tab":
		return "shift+tab", nil
	case "esc", "escape":
		return "esc", nil
	case "enter", "return":
		return "enter", nil
	case "space":
		return " ", nil
	case "up":
		return "up", nil
	case "down":
		return "down", nil
	case "left":
		return "left", nil
	case "right":
		return "right", nil
	case "pgup", "pageup", "pg up":
		return "pgup", nil
	case "pgdown", "pgdn", "pagedown", "pg down":
		return "pgdown", nil
	case "home":
		return "home", nil
	case "end":
		return "end", nil
	}

	if utf8.RuneCountInString(s) == 1 {
		return s, nil
	}
	return "", fmt.Errorf("unsupported key %q", spec)
}
