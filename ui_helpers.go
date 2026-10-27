package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

// This file replaces the old helpers that wrote directly into a tview.TextView.
// Everything here returns a styled string; the caller decides where it goes.

// helpLine renders one "key = description" row.
func helpLine(th *theme, key, desc string) string {
	return " " + th.HelpKey.Render(fmt.Sprintf("%-30s", key)) + th.HelpDim.Render("= ") + desc
}

// helpSection renders a bold section heading.
func helpSection(th *theme, title string) string {
	return th.HelpHeading.Render(title)
}

// helpText builds the full help screen, reading bindings from config so the
// output always matches what is actually wired up.
func helpText(th *theme) string {
	cmdPrefix := config.Config.General.CmdPrefix
	km := config.Config.Keymap

	var b strings.Builder
	line := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	line("")
	line(helpSection(th, "Application & Connection"))
	line(helpLine(th, cmdPrefix+"connect / "+km.CommandConnect, "(Re)Connect"))
	line(helpLine(th, cmdPrefix+"disconnect", "Disconnect"))
	line(helpLine(th, cmdPrefix+"logout", "Logout & Delete Data"))
	line(helpLine(th, cmdPrefix+"quit / "+km.CommandQuit, "Exit"))
	line("")

	line(helpSection(th, "Navigation & View"))
	line(helpLine(th, "Up/Down", "Scroll Chats/History"))
	line(helpLine(th, km.SwitchPanels, "Switch Panel"))
	line(helpLine(th, km.FocusMessages, "Focus Messages"))
	line(helpLine(th, "Ctrl+p", "Toggle Mouse"))
	line("")

	line(helpSection(th, "Chat & Messages"))
	line(helpLine(th, cmdPrefix+"backlog / "+km.CommandBacklog, "Load Backlog"))
	line(helpLine(th, cmdPrefix+"read / "+km.CommandRead, "Mark Read"))
	line(helpLine(th, "Up/Down", "Select Message"))
	line(helpLine(th, km.MessageDownload, "Download"))
	line(helpLine(th, km.MessageOpen, "Download & Open"))
	line(helpLine(th, km.MessageShow, "Show Image ("+config.Config.General.ShowCommand+")"))
	line(helpLine(th, km.MessageUrl, "Open URL"))
	line(helpLine(th, km.MessageRevoke, "Revoke"))
	line(helpLine(th, km.MessageInfo, "Info"))
	line("")

	line(helpSection(th, "Media"))
	line(helpLine(th, cmdPrefix+"upload <path>", "Send File"))
	line(helpLine(th, cmdPrefix+"sendimage <path>", "Send Image"))
	line(helpLine(th, cmdPrefix+"sendvideo <path>", "Send Video"))
	line(helpLine(th, cmdPrefix+"sendaudio <path>", "Send Audio"))
	line("")

	line(helpSection(th, "Search"))
	line(helpLine(th, cmdPrefix+"search <keyword>", "Search Messages"))
	line(helpLine(th, cmdPrefix+"search-contact <name>", "Find Contacts"))
	line("")

	line(helpSection(th, "Groups"))
	line(helpLine(th, cmdPrefix+"create <nums> <subj>", "Create"))
	line(helpLine(th, cmdPrefix+"subject <subj>", "Subject"))
	line(helpLine(th, cmdPrefix+"leave", "Leave"))
	line(helpLine(th, cmdPrefix+"add <userid>", "Add"))
	line(helpLine(th, cmdPrefix+"remove <userid>", "Kick"))
	line(helpLine(th, cmdPrefix+"admin <userid>", "Promote"))
	line(helpLine(th, cmdPrefix+"removeadmin <userid>", "Demote"))
	line("")

	line(helpSection(th, "Clipboard"))
	line(helpLine(th, km.Copyuser, "Copy UserID"))
	line(helpLine(th, km.Pasteuser, "Paste UserID"))
	line("")

	line("Config: " + config.GetConfigFilePath())
	line("Verify: " + VERSION)
	return b.String()
}

// statusBarText renders the top bar: product name on the left, session status on
// the right. It replaces UpdateStatusBar, which wrote straight into the TextView.
func statusBarText(st messages.SessionStatus, width int, th *theme) string {
	left := lipgloss.NewStyle().Bold(true).Foreground(th.Text).Render(" WhatsCLI " + VERSION)
	hint := th.HelpDim.Render("  Type " + config.Config.General.CmdPrefix + "help or press " + config.Config.Keymap.CommandHelp + " for help")

	var status strings.Builder
	status.WriteString(" ")
	if st.Connected {
		status.WriteString(lipgloss.NewStyle().Foreground(th.Positive).Render("online"))
	} else {
		status.WriteString(lipgloss.NewStyle().Foreground(th.Negative).Render("offline"))
	}
	status.WriteString(th.HelpDim.Render(" ("))
	status.WriteString(fmt.Sprint(st.BatteryCharge))
	status.WriteString("%")
	if st.BatteryLoading {
		status.WriteString(" " + lipgloss.NewStyle().Foreground(th.Positive).Render("L"))
	} else {
		status.WriteString(" " + lipgloss.NewStyle().Foreground(th.Negative).Render("l"))
	}
	if st.BatteryPowersave {
		status.WriteString(" " + lipgloss.NewStyle().Foreground(th.Negative).Render("S"))
	} else {
		status.WriteString(" " + lipgloss.NewStyle().Foreground(th.Positive).Render("s"))
	}
	status.WriteString(th.HelpDim.Render(") "))
	if st.LastSeen != "" {
		status.WriteString(st.LastSeen)
	}
	status.WriteString(" ")
	right := status.String()

	// Drop the hint before the status when space is tight.
	if lipgloss.Width(left)+lipgloss.Width(hint)+lipgloss.Width(right) > width {
		hint = ""
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(hint) - lipgloss.Width(right)
	if gap < 0 {
		return fit(left+right, width)
	}
	return left + hint + strings.Repeat(" ", gap) + right
}

// emptyChatText is shown when a chat has no messages, matching the previous
// NewScreen hint.
func emptyChatText(th *theme) string {
	return th.HelpDim.Render(" ~~~ no messages, press " + config.Config.Keymap.CommandBacklog + " to load backlog if available ~~~")
}

// errorText renders an error line for the message pane.
func errorText(th *theme, err error) string {
	if err == nil {
		return ""
	}
	return th.Error.Render(err.Error())
}
