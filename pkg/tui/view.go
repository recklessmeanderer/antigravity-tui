package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Initializing Antigravity TUI..."
	}

	sidebarWidth := 34
	contentWidth := m.width - sidebarWidth - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// 1. Top Header
	title := headerStyle.Render("⚡ ANTIGRAVITY REMOTE TUI")
	connBadge := badgeConnected.Render(fmt.Sprintf("● CONNECTED :%d", m.hubInfo.HttpPort))
	hubPID := badgeIdle.Render(fmt.Sprintf("PID %d", m.hubInfo.Pid))
	statusText := lipgloss.NewStyle().Foreground(subduedColor).Render(m.statusMsg)

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center, title, " ", connBadge, " ", hubPID, "  ", statusText)

	// 2. Sidebar View
	sidebarContent := m.renderSidebar(sidebarWidth)
	sbStyle := paneStyle
	if m.focusedPane == FocusSidebar {
		sbStyle = activePaneStyle
	}
	sidebarView := sbStyle.
		Width(sidebarWidth).
		Height(m.height - 4).
		Render(sidebarContent)

	// 3. Right Pane (Chat Viewport + Input Box)
	// Active conversation title bar
	convTitle := "No session selected"
	convStatus := ""
	if len(m.trajectories) > 0 && m.selectedIdx < len(m.trajectories) {
		active := m.trajectories[m.selectedIdx]
		if active.Summary.Annotations != nil && active.Summary.Annotations.Title != "" {
			convTitle = active.Summary.Annotations.Title
		} else if active.Summary.Summary != "" {
			convTitle = active.Summary.Summary
		} else {
			convTitle = active.CascadeID
		}

		if active.Summary.Status == "CASCADE_RUN_STATUS_RUNNING" {
			convStatus = badgeRunning.Render("● AGENT RUNNING " + m.spinner.View())
		} else {
			convStatus = badgeIdle.Render(active.Summary.Status)
		}
	}

	chatHeader := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(textColor).Render(convTitle),
		"  ",
		convStatus,
	)

	// Chat Viewport
	vpStyle := paneStyle
	if m.focusedPane == FocusChat {
		vpStyle = activePaneStyle
	}
	vpView := vpStyle.
		Width(contentWidth).
		Height(m.viewport.Height).
		Render(m.viewport.View())

	// Input Box
	inStyle := paneStyle
	if m.focusedPane == FocusInput {
		inStyle = activePaneStyle
	}
	inView := inStyle.
		Width(contentWidth).
		Render(m.input.View())

	rightPane := lipgloss.JoinVertical(lipgloss.Left,
		chatHeader,
		vpView,
		inView,
	)

	// 4. Main Body Split
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, " ", rightPane)

	// 5. Bottom Statusline
	statusline := statusBarStyle.Width(m.width).Render(
		keybindStyle.Render("[Tab]") + " Switch Pane  " +
			keybindStyle.Render("[↑/↓]") + " Select/Scroll  " +
			keybindStyle.Render("[Enter]") + " Send Message  " +
			keybindStyle.Render("[r]") + " Refresh  " +
			keybindStyle.Render("[Esc]") + " Stop / Unfocus  " +
			keybindStyle.Render("[Ctrl+C]") + " Quit",
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerLine,
		body,
		statusline,
	)
}

func (m Model) renderSidebar(width int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render("SESSIONS / TRAJECTORIES"))
	b.WriteString("\n\n")

	if len(m.trajectories) == 0 {
		b.WriteString(normalItemStyle.Render("No active conversations found."))
		return b.String()
	}

	for i, item := range m.trajectories {
		name := item.Summary.Summary
		if item.Summary.Annotations != nil && item.Summary.Annotations.Title != "" {
			name = item.Summary.Annotations.Title
		}
		if name == "" {
			name = item.CascadeID[:12] + "..."
		}

		// Truncate name to fit width
		maxLen := width - 6
		if maxLen > 4 && len(name) > maxLen {
			name = name[:maxLen-3] + "..."
		}

		statusPill := "•"
		if item.Summary.Status == "CASCADE_RUN_STATUS_RUNNING" {
			statusPill = "▶"
		}

		line := fmt.Sprintf("%s %s", statusPill, name)
		meta := fmt.Sprintf("  %d steps", item.Summary.StepCount)

		if i == m.selectedIdx {
			b.WriteString(selectedItemStyle.Width(width - 4).Render("❯ " + line))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#A0AEC0")).Render(meta))
			b.WriteString("\n\n")
		} else {
			b.WriteString(normalItemStyle.Width(width - 4).Render("  " + line))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#555A64")).Render(meta))
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

func (m Model) renderTranscript() string {
	if len(m.steps) == 0 {
		return lipgloss.NewStyle().Foreground(subduedColor).Render("No messages in this conversation yet.")
	}

	var b strings.Builder

	for _, step := range m.steps {
		switch step.Type {
		case "CORTEX_STEP_TYPE_USER_INPUT":
			text := ""
			if step.UserInput != nil {
				if step.UserInput.UserResponse != "" {
					text = step.UserInput.UserResponse
				} else if len(step.UserInput.Items) > 0 {
					text = step.UserInput.Items[0].Text
				}
			}
			b.WriteString(userHeaderStyle.Render("👤 USER"))
			b.WriteString("\n")
			b.WriteString(userBoxStyle.Render(text))
			b.WriteString("\n")

		case "CORTEX_STEP_TYPE_PLANNER_RESPONSE":
			if step.PlannerResponse != nil {
				b.WriteString(agentHeaderStyle.Render("🤖 ANTIGRAVITY AGENT"))
				b.WriteString("\n")

				if step.PlannerResponse.Thinking != "" {
					lines := strings.Split(strings.TrimSpace(step.PlannerResponse.Thinking), "\n")
					var shortThinking []string
					for _, l := range lines {
						if strings.TrimSpace(l) != "" {
							shortThinking = append(shortThinking, "  💭 "+l)
						}
					}
					if len(shortThinking) > 0 {
						b.WriteString(thinkingStyle.Render(strings.Join(shortThinking, "\n")))
						b.WriteString("\n\n")
					}
				}

				for _, call := range step.PlannerResponse.ToolCalls {
					b.WriteString(toolCallStyle.Render(fmt.Sprintf("  🔧 [Tool: %s]", call.Name)))
					b.WriteString("\n")
					if call.ArgumentsJSON != "" {
						b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("     " + call.ArgumentsJSON))
						b.WriteString("\n")
					}
				}
				b.WriteString("\n")
			}

		case "CORTEX_STEP_TYPE_RUN_COMMAND":
			if step.RunCommand != nil {
				b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4FD1C5")).Render("  ▶ EXECUTED COMMAND:"))
				b.WriteString("\n")
				b.WriteString(commandOutputStyle.Render("$ " + step.RunCommand.CommandLine))
				b.WriteString("\n")
				if step.RunCommand.Output != "" {
					outLines := strings.Split(step.RunCommand.Output, "\n")
					if len(outLines) > 10 {
						outLines = append(outLines[:10], "... [truncated]")
					}
					b.WriteString(commandOutputStyle.Render(strings.Join(outLines, "\n")))
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}
