package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/normen/whatscli/messages"
)

// AppContext keeps the long-lived application objects together. It replaces the
// widget-holding context the tview version needed; only the program, the backend
// session manager and the message bridge remain.
type AppContext struct {
	Program        *tea.Program
	UiHandler      UiHandler
	SessionManager *messages.SessionManager
	Bridge         *uiBridge
}

// ctx is the single application context, kept for parity with the previous
// design where command handlers reached shared state through it.
var ctx *AppContext
