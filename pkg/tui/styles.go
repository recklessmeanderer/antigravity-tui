package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Base Colors
	primaryColor   = lipgloss.Color("#7F5AF0") // Electric purple / indigo
	secondaryColor = lipgloss.Color("#2CB67D") // Mint green
	accentColor    = lipgloss.Color("#FF8906") // Vibrant amber
	textColor      = lipgloss.Color("#FFFFFE") // Off white
	subduedColor   = lipgloss.Color("#94A1B2") // Steel gray
	bgColor        = lipgloss.Color("#16161A") // Deep dark
	surfaceColor   = lipgloss.Color("#242629") // Card surface
	activeBorder   = lipgloss.Color("#7F5AF0")
	inactiveBorder = lipgloss.Color("#44475A")
	errorColor     = lipgloss.Color("#E53E3E")

	// Header Styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(surfaceColor).
			Padding(0, 1)

	badgeConnected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#16161A")).
			Background(secondaryColor).
			Padding(0, 1)

	badgeIdle = lipgloss.NewStyle().
			Foreground(subduedColor).
			Background(surfaceColor).
			Padding(0, 1)

	badgeRunning = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#16161A")).
			Background(accentColor).
			Padding(0, 1)

	// Panel Styles
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(inactiveBorder).
			Padding(0, 1)

	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorder).
			Padding(0, 1)

	// Project Tree Styles
	projectHeaderNormal = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#CBD5E1")).
				Background(lipgloss.Color("#1E212B")).
				Padding(0, 1)

	projectHeaderSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor).
				Background(primaryColor).
				Padding(0, 1)

	iconExpanded = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	iconCollapsed = lipgloss.NewStyle().
			Bold(true).
			Foreground(subduedColor)

	// Conversation List Items
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor).
				Background(lipgloss.Color("#2E2F3E")).
				Padding(0, 1)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(subduedColor).
			Padding(0, 1)

	// Chat Message Styles
	userHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	userBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(secondaryColor).
			PaddingLeft(1).
			MarginTop(1).
			MarginBottom(1)

	agentHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	agentBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(primaryColor).
			PaddingLeft(1).
			MarginTop(1).
			MarginBottom(1)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#72757D")).
			Italic(true)

	toolCallStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	commandOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CCCCCC")).
				Background(lipgloss.Color("#1E1E24")).
				Padding(0, 1)

	// Footer Help Style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(subduedColor).
			Background(surfaceColor).
			Padding(0, 1)

	keybindStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)
)
