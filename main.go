package main

import (
	"flag"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/normen/whatscli/config"
	"github.com/normen/whatscli/messages"
)

var VERSION string = "v1.3.0"

const batchSize = 50

func main() {
	// T-2: Parse --debug flag before anything else.
	debugMode := flag.Bool("debug", false, "Enable verbose WhatsApp protocol logging")
	flag.Parse()

	err := config.InitConfig()
	if err != nil {
		fmt.Printf("Failed to initialize config: %v\n", err)
		return
	}

	th, err := loadTheme(*config.Config.Colors)
	if err != nil {
		fmt.Printf("Failed to load colors: %v\n", err)
		return
	}

	keys, keyErr := buildKeyMap(*config.Config.Keymap)

	ctx = &AppContext{Bridge: &uiBridge{}}
	ctx.SessionManager = &messages.SessionManager{}
	ctx.SessionManager.SetDebug(*debugMode)
	ctx.UiHandler = UiHandler{bridge: ctx.Bridge, th: th}

	model := newUIModel(th, ctx.SessionManager, keys)
	if keyErr != nil {
		model.msgs.addText(errorText(th, fmt.Errorf("invalid key binding: %w", keyErr)), th)
	}

	ctx.Program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithoutCatchPanics())
	ctx.Bridge.program = ctx.Program

	if err := ctx.SessionManager.Init(ctx.UiHandler); err != nil {
		fmt.Printf("Failed to initialize session: %v\n", err)
		return
	}
	if err := ctx.SessionManager.StartManager(); err != nil {
		ctx.Bridge.send(msgError{err: err})
	}

	if _, err := ctx.Program.Run(); err != nil {
		fmt.Printf("UI error: %v\n", err)
	}
	ctx.SessionManager.Shutdown()
}
