package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/normen/whatscli/messages"
)

// This file holds pure formatting helpers: they take data plus a theme and
// return styled strings. No model state, no terminal I/O — easy to unit test.

// formatRelativeTime formats a Unix timestamp into a WhatsApp Web–style
// relative label: "just now", "5m", "2h", "Yesterday", weekday, or "Jan 02".
func formatRelativeTime(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	now := time.Now()
	t := time.Unix(timestamp, 0)
	diff := now.Sub(t)

	// Guard against future timestamps (clock skew / server-side drift)
	if diff < 0 {
		return "just now"
	}

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	case diff < 24*time.Hour:
		// Same calendar day check
		yN, mN, dN := now.Date()
		yT, mT, dT := t.Date()
		if yN == yT && mN == mT && dN == dT {
			return fmt.Sprintf("%dh", int(diff.Hours()))
		}
		return "Yesterday"
	case diff < 48*time.Hour:
		return "Yesterday"
	case diff < 7*24*time.Hour:
		return t.Weekday().String()[:3] // "Mon", "Tue", etc.
	default:
		// "Jan 02" for everything older. A year suffix ("Sep 13, 24") is 10
		// cells and truncates to "Sep 13, 20" at the default 30-cell sidebar,
		// which is worse than the ambiguity (plan item C2); the pane shows the
		// full date on the message header anyway.
		return t.Format("Jan 02")
	}
}

// chatRowName builds the display name for a conversation, including the pinned
// marker and the unread counter.
func chatRowName(conv *messages.Conversation, th *theme) string {
	name := conv.Name
	if name == "" {
		name = conv.JID // Fallback
	}
	out := name
	if conv.IsPinned {
		out = "📌 " + out
	}
	if conv.Unread > 0 {
		out += " " + th.Unread.Render(fmt.Sprintf("(%d)", conv.Unread))
	}
	return out
}

// renderChatRow renders one sidebar entry to exactly w cells. When selected the
// whole row is reverse-video so the cursor is visible across the full width.
func renderChatRow(conv *messages.Conversation, selected bool, color lipgloss.Color, w int, th *theme) string {
	if w <= 0 {
		return ""
	}
	ts := formatRelativeTime(conv.LastMsgTime)
	tsW := 0
	if ts != "" {
		tsW = lipgloss.Width(ts) + 1 // one space before the timestamp
	}
	nameW := w - tsW
	if nameW < 4 {
		// Not enough room for a timestamp; drop it.
		ts, tsW = "", 0
		nameW = w
	}

	name := chatRowName(conv, th)
	namePlain := ansiTruncate(name, nameW)
	namePadded := namePlain + strings.Repeat(" ", max(0, nameW-lipgloss.Width(namePlain)))

	if selected {
		row := namePadded
		if tsW > 0 {
			row += " " + ts
		}
		return th.Selected.Render(padTo(row, w))
	}

	paneColor := lipgloss.NewStyle().Foreground(color)
	row := paneColor.Render(namePadded)
	if tsW > 0 {
		row += " " + th.TimeDim.Render(ts)
	}
	return row
}

// renderMessage renders one chat message: dim timestamp, bold coloured sender,
// then the message body (tinted when forwarded).
func renderMessage(msg *messages.Message, th *theme) string {
	ts := time.Unix(int64(msg.Timestamp), 0).Format("02-01-06 15:04:05")
	out := th.TimeDim.Render("(" + ts + ") ")

	sender := "Me"
	senderColor := th.ChatMe
	if !msg.FromMe {
		sender = msg.ContactShort
		if sender == "" {
			sender = msg.ContactName
		}
		senderColor = th.ChatContact
	}
	out += lipgloss.NewStyle().Foreground(senderColor).Bold(true).Render(sender) + ": "

	textColor := th.Text
	if msg.Forwarded {
		textColor = th.ForwardedText
	}
	out += lipgloss.NewStyle().Foreground(textColor).Render(msg.Text)
	return out
}

// ansiTruncate truncates styled text to w display cells, appending an ellipsis
// only when something was actually removed.
func ansiTruncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return fit(s, w)
}
