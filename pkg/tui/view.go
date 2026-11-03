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

	sidebarWidth := 36
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
	convTitle := "No session selected"
	convStatus := ""
	for _, conv := range m.conversations {
		if conv.CascadeID == m.activeCascadeID {
			if conv.Title != "" {
				convTitle = conv.Title
			} else {
				convTitle = conv.CascadeID
			}

			if conv.Status == "CASCADE_RUN_STATUS_RUNNING" {
				convStatus = badgeRunning.Render("● AGENT RUNNING " + m.spinner.View())
			} else {
				convStatus = badgeIdle.Render(conv.Status)
			}
			break
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
		keybindStyle.Render("[Tab]") + " Pane  " +
			keybindStyle.Render("[Enter/Space]") + " Expand/Open  " +
			keybindStyle.Render("[PgUp/PgDn / MouseWheel]") + " Scroll  " +
			keybindStyle.Render("[r]") + " Refresh  " +
			keybindStyle.Render("[Esc]") + " Stop/Back  " +
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
	header := fmt.Sprintf("PROJECTS (%d)", len(m.projectGroups))
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(header))
	b.WriteString("\n")

	if len(m.visibleNodes) == 0 {
		b.WriteString("\n" + normalItemStyle.Render("No conversations found.\nPress 'r' to refresh."))
		return b.String()
	}

	maxVisible := m.height - 7
	if maxVisible < 3 {
		maxVisible = 3
	}

	start := m.sidebarOffset
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > len(m.visibleNodes) {
		end = len(m.visibleNodes)
	}

	// Scroll indicator top
	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("  ▲ %d more above\n", start)))
	} else {
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		node := m.visibleNodes[i]
		isSelected := (i == m.selectedNodeIdx)

		if node.IsHeader {
			isExpanded := m.expandedState[node.ProjectName]
			icon := "▶"
			if isExpanded {
				icon = "▼"
			}
			label := fmt.Sprintf("%s %s (%d)", icon, node.ProjectName, node.Count)

			maxLen := width - 4
			if len(label) > maxLen {
				label = label[:maxLen-3] + "..."
			}

			if isSelected {
				b.WriteString(projectHeaderSelected.Width(width - 4).Render(label))
			} else {
				b.WriteString(projectHeaderNormal.Width(width - 4).Render(label))
			}
			b.WriteString("\n")
		} else if node.Conversation != nil {
			conv := node.Conversation
			name := conv.Title
			if name == "" {
				name = conv.CascadeID[:10] + "..."
			}

			statusPill := "•"
			if conv.Status == "CASCADE_RUN_STATUS_RUNNING" {
				statusPill = "▶"
			}

			line := fmt.Sprintf("  %s %s", statusPill, name)
			maxLen := width - 4
			if len(line) > maxLen {
				line = line[:maxLen-3] + "..."
			}

			if isSelected {
				b.WriteString(selectedItemStyle.Width(width - 4).Render("❯" + line[1:]))
			} else {
				b.WriteString(normalItemStyle.Width(width - 4).Render(line))
			}
			b.WriteString("\n")
		}
	}

	// Scroll indicator bottom
	if end < len(m.visibleNodes) {
		b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("  ▼ %d more below", len(m.visibleNodes)-end)))
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
			b.WriteString("\n\n")

		case "CORTEX_STEP_TYPE_PLANNER_RESPONSE":
			if step.PlannerResponse != nil {
				// 1. Thinking block
				if step.PlannerResponse.Thinking != "" {
					lines := strings.Split(strings.TrimSpace(step.PlannerResponse.Thinking), "\n")
					var shortThinking []string
					for _, l := range lines {
						trimmed := strings.TrimSpace(l)
						if trimmed != "" {
							shortThinking = append(shortThinking, "  💭 "+trimmed)
						}
					}
					if len(shortThinking) > 0 {
						b.WriteString(thinkingStyle.Render(strings.Join(shortThinking, "\n")))
						b.WriteString("\n\n")
					}
				}

				// 2. Tool calls
				for _, call := range step.PlannerResponse.ToolCalls {
					b.WriteString(toolCallStyle.Render(fmt.Sprintf("  🔧 [Tool: %s]", call.Name)))
					b.WriteString("\n")
					if call.ArgumentsJSON != "" {
						b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("     " + call.ArgumentsJSON))
						b.WriteString("\n")
					}
				}

				// 3. Model visible text response
				if step.PlannerResponse.Response != "" {
					b.WriteString(agentHeaderStyle.Render("🤖 ANTIGRAVITY AGENT"))
					b.WriteString("\n")
					b.WriteString(agentBoxStyle.Render(strings.TrimSpace(step.PlannerResponse.Response)))
					b.WriteString("\n\n")
				}
			}

		case "CORTEX_STEP_TYPE_RUN_COMMAND":
			if step.RunCommand != nil {
				b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4FD1C5")).Render("  ▶ EXECUTED COMMAND:"))
				b.WriteString("\n")
				b.WriteString(commandOutputStyle.Render("$ " + step.RunCommand.CommandLine))
				b.WriteString("\n")
				if step.RunCommand.Output != "" {
					outLines := strings.Split(step.RunCommand.Output, "\n")
					if len(outLines) > 12 {
						outLines = append(outLines[:12], "... [truncated]")
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
