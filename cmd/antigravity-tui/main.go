package main

import (
	"flag"
	"fmt"
	"os"

	"antigravity-tui/pkg/hub"
	"antigravity-tui/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		configPath = flag.String("config", "", "Path to Antigravity hub JSON file (default auto-discovers in ~/.gemini/antigravity/daemon)")
		portFlag   = flag.Int("port", 0, "Override Antigravity HTTP port")
		tokenFlag  = flag.String("token", "", "Override Antigravity CSRF token")
	)
	flag.Parse()

	// 1. Auto-discover active local Antigravity hub daemon
	hubInfo, err := hub.DiscoverHub(*configPath)
	if err != nil && (*portFlag == 0 || *tokenFlag == "") {
		fmt.Fprintf(os.Stderr, "Error discovering Antigravity Hub: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure Antigravity is running, or specify -port and -token manually.\n")
		os.Exit(1)
	}

	if hubInfo == nil {
		hubInfo = &hub.HubInfo{}
	}
	if *portFlag > 0 {
		hubInfo.HttpPort = *portFlag
	}
	if *tokenFlag != "" {
		hubInfo.CsrfToken = *tokenFlag
	}

	// 2. Connect to Antigravity WebSocket RPC
	client, err := hub.NewClient(hubInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to Antigravity Hub: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// 3. Launch Bubble Tea interactive TUI
	model := tui.NewModel(client, hubInfo)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
