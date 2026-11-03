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
	bodyHeight := m.height - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	sbHeight := bodyHeight - 2
	if sbHeight < 1 {
		sbHeight = 1
	}
	sidebarContent := m.renderSidebar(sidebarWidth)
	sbStyle := paneStyle
	if m.focusedPane == FocusSidebar {
		sbStyle = activePaneStyle
	}
	sidebarView := sbStyle.
		Width(sidebarWidth).
		Height(sbHeight).
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

	// 5. Bottom Clickable Button Statusline
	buttonBar, _ := m.renderButtonBar()
	statusline := statusBarStyle.Width(m.width).Render(buttonBar)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerLine,
		body,
		statusline,
	)
}

func (m Model) renderButtonBar() (string, []ButtonHitbox) {
	if m.modalMode == ModalNewProject {
		prompt := lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("📁 NEW PROJECT: ")
		inputTxt := lipgloss.NewStyle().Background(surfaceColor).Foreground(textColor).Render(" " + m.modalInput + "█ ")
		help := lipgloss.NewStyle().Foreground(subduedColor).Render(" [Press Enter to Create, Esc to Cancel]")
		return prompt + inputTxt + help, nil
	}

	if m.modalMode == ModalConfirmDelete {
		prompt := lipgloss.NewStyle().Bold(true).Foreground(errorColor).Render(m.statusMsg + " [Press 'y' to Confirm, 'n' or Esc to Cancel]")
		return prompt, nil
	}

	var hitboxes []ButtonHitbox
	var renderedBtns []string
	currentX := 0

	addBtn := func(id, text string, style lipgloss.Style) {
		rendered := style.Render(text)
		width := lipgloss.Width(rendered)
		hitboxes = append(hitboxes, ButtonHitbox{
			ID:   id,
			MinX: currentX,
			MaxX: currentX + width - 1,
		})
		renderedBtns = append(renderedBtns, rendered)
		currentX += width + 1 // + 1 for separator space
	}

	// 1. Tab / Pane Button
	tabText := "⇥ Pane"
	tStyle := btnStyle
	if m.focusedPane == FocusSidebar {
		tabText = "⇥ Sidebar"
		tStyle = btnActiveStyle
	} else if m.focusedPane == FocusChat {
		tabText = "⇥ Chat"
		tStyle = btnActiveStyle
	} else if m.focusedPane == FocusInput {
		tabText = "⇥ Input"
		tStyle = btnActiveStyle
	}
	addBtn("tab", tabText, tStyle)

	// 2. Open / Expand Button
	addBtn("open", "␣ Open", btnStyle)

	// 3. New Chat Button
	addBtn("new_chat", "+ Chat (Ctrl+N)", btnStyle)

	// 4. New Project Button
	addBtn("new_proj", "+ Proj (Ctrl+P)", btnStyle)

	// 5. Delete Button (Context-aware: Conversation vs Project)
	delLabel := "🗑 Del (Ctrl+X)"
	if len(m.visibleNodes) > 0 && m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
		if m.visibleNodes[m.selectedNodeIdx].IsHeader {
			delLabel = "🗑 Del Proj (Ctrl+X)"
		}
	}
	addBtn("del", delLabel, btnDangerStyle)

	// 6. Refresh Button
	addBtn("refresh", "⟳ Refresh", btnStyle)

	// 7. Quit Button
	addBtn("quit", "✕ Quit (Ctrl+C)", btnQuitStyle)

	buttonBar := strings.Join(renderedBtns, " ")
	return buttonBar, hitboxes
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

	bodyHeight := m.height - 2
	maxVisible := bodyHeight - 5
	if maxVisible < 1 {
		maxVisible = 1
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
		if m.activeCascadeID != "" {
			return lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history...")
		}
		return lipgloss.NewStyle().Foreground(subduedColor).Render("Select a conversation from the sidebar to view history.")
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
				output := step.RunCommand.GetOutput()
				if output != "" {
					outLines := strings.Split(output, "\n")
					if len(outLines) > 12 {
						outLines = append(outLines[:12], "... [truncated]")
					}
					b.WriteString(commandOutputStyle.Render(strings.Join(outLines, "\n")))
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}

		case "CORTEX_STEP_TYPE_VIEW_FILE":
			if step.ViewFile != nil && step.ViewFile.AbsolutePathURI != "" {
				b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("  👁 [View File: %s]\n\n", step.ViewFile.AbsolutePathURI)))
			}

		case "CORTEX_STEP_TYPE_SEARCH_WEB":
			if step.SearchWeb != nil && step.SearchWeb.Query != "" {
				b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("  🌐 [Web Search: %s]\n\n", step.SearchWeb.Query)))
			}

		case "CORTEX_STEP_TYPE_MCP_TOOL":
			if step.McpTool != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("  🔌 [MCP: %s / %s]\n\n", step.McpTool.ServerName, step.McpTool.ToolCall.Name)))
			}
		}
	}

	if b.Len() == 0 {
		return lipgloss.NewStyle().Foreground(subduedColor).Render("No displayable messages in this conversation yet.")
	}

	return b.String()
}
