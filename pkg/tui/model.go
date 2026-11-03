package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"antigravity-tui/pkg/hub"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paneFocus int

const (
	FocusSidebar paneFocus = iota
	FocusInput
	FocusChat
)

type ModalMode int

const (
	ModalNone ModalMode = iota
	ModalNewProject
	ModalConfirmDelete
)

type ButtonHitbox struct {
	ID   string
	MinX int
	MaxX int
}

type Model struct {
	client           *hub.Client
	hubInfo          *hub.HubInfo
	conversations    []hub.ConversationItem
	projectGroups    []*ProjectGroup
	visibleNodes     []SidebarNode
	selectedNodeIdx  int
	sidebarOffset    int
	expandedState    map[string]bool
	activeCascadeID  string
	steps            []hub.Step
	viewport         viewport.Model
	input            textarea.Model
	spinner          spinner.Model
	focusedPane      paneFocus
	statusMsg        string
	isStreaming      bool
	streamCancelFunc func()
	width            int
	height           int
	err              error
	lastUpdate       time.Time

	// Modal / interactive action states
	modalMode            ModalMode
	modalInput           string
	deleteTargetName     string
	deleteTargetID       string
	deleteIsProject      bool
	createdConversations map[string]hub.ConversationItem
	deletedCascadeIDs    map[string]bool
}

// Msg types
type conversationsLoadedMsg struct {
	items []hub.ConversationItem
}

type conversationDeletedMsg struct {
	cascadeID string
	title     string
}

type projectDeletedMsg struct {
	projectName string
}

type stepsLoadedMsg struct {
	cascadeID string
	steps     []hub.Step
}

type errMsg struct {
	err error
}

type periodicTickMsg time.Time

func NewModel(client *hub.Client, hubInfo *hub.HubInfo) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message or instruction to the agent... (Enter to send, Shift+Enter for newline)"
	ta.Focus()
	ta.Prompt = "❯ "
	ta.CharLimit = 16000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Connecting and loading conversations...")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return Model{
		client:          client,
		hubInfo:         hubInfo,
		conversations:   nil,
		expandedState:   make(map[string]bool), // all start collapsed by default
		selectedNodeIdx: 0,
		sidebarOffset:   0,
		viewport:        vp,
		input:           ta,
		spinner:              sp,
		focusedPane:          FocusInput,
		statusMsg:            "Connected to Antigravity Hub",
		lastUpdate:           time.Now(),
		createdConversations: make(map[string]hub.ConversationItem),
		deletedCascadeIDs:    make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadConversationsCmd(),
		m.periodicTickCmd(),
	)
}

func (m Model) periodicTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return periodicTickMsg(t)
	})
}

func (m Model) loadConversationsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items, err := m.client.ListAllConversations(ctx)
		if err != nil {
			return errMsg{err: err}
		}

		return conversationsLoadedMsg{items: items}
	}
}

func (m Model) loadStepsCmd(cascadeID string) tea.Cmd {
	return func() tea.Msg {
		if cascadeID == "" {
			return nil
		}
		// If this is a locally created conversation that has 0 steps on disk,
		// don't call GetTrajectorySteps which hangs waiting for non-existent server cascade!
		if _, ok := m.createdConversations[cascadeID]; ok {
			diskSteps, err := hub.LoadTranscriptFromDisk(cascadeID)
			if err != nil || len(diskSteps) == 0 {
				return stepsLoadedMsg{
					cascadeID: cascadeID,
					steps:     nil,
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		steps, err := m.client.GetTrajectorySteps(ctx, cascadeID)
		if err != nil {
			return errMsg{err: err}
		}
		return stepsLoadedMsg{
			cascadeID: cascadeID,
			steps:     steps,
		}
	}
}

func (m Model) sendMessageCmd(text string) tea.Cmd {
	cascadeID := m.activeCascadeID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.client.SendMessage(ctx, cascadeID, text)
		if err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m Model) cancelCascadeCmd() tea.Cmd {
	cascadeID := m.activeCascadeID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = m.client.CancelCascade(ctx, cascadeID)
		return nil
	}
}

func (m *Model) startStreamingUpdates(cascadeID string) tea.Cmd {
	if m.streamCancelFunc != nil {
		m.streamCancelFunc()
		m.streamCancelFunc = nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		subID := fmt.Sprintf("tui-%d", time.Now().UnixNano())
		cancelFn, err := m.client.StreamAgentStateUpdates(ctx, cascadeID, subID, func(raw json.RawMessage) {
			// Streaming update callback
		})
		if err != nil {
			return errMsg{err: err}
		}
		m.streamCancelFunc = cancelFn
		m.isStreaming = true
		return nil
	}
}

func (m *Model) rebuildTree() {
	m.projectGroups, m.visibleNodes = BuildProjectGroups(m.conversations, m.expandedState)

	if len(m.visibleNodes) == 0 {
		m.selectedNodeIdx = 0
		m.sidebarOffset = 0
		return
	}

	if m.selectedNodeIdx >= len(m.visibleNodes) {
		m.selectedNodeIdx = len(m.visibleNodes) - 1
	}
	if m.selectedNodeIdx < 0 {
		m.selectedNodeIdx = 0
	}

	// Keep sidebar scroll in view
	bodyHeight := m.height - 2
	maxVisible := bodyHeight - 5
	if maxVisible < 1 {
		maxVisible = 1
	}

	if m.selectedNodeIdx < m.sidebarOffset {
		m.sidebarOffset = m.selectedNodeIdx
	}
	if m.selectedNodeIdx >= m.sidebarOffset+maxVisible {
		m.sidebarOffset = m.selectedNodeIdx - maxVisible + 1
	}
	if m.sidebarOffset < 0 {
		m.sidebarOffset = 0
	}
}

func (m *Model) cyclePaneFocus() {
	switch m.focusedPane {
	case FocusSidebar:
		m.focusedPane = FocusInput
		m.input.Focus()
	case FocusInput:
		m.focusedPane = FocusChat
		m.input.Blur()
	case FocusChat:
		m.focusedPane = FocusSidebar
		m.input.Blur()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	sidebarWidth := 36

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if msg.X < sidebarWidth {
				if m.sidebarOffset > 0 {
					m.sidebarOffset--
				}
			} else {
				m.viewport.LineUp(3)
			}

		case tea.MouseWheelDown:
			if msg.X < sidebarWidth {
				maxVisible := m.height - 6
				if m.sidebarOffset+maxVisible < len(m.visibleNodes) {
					m.sidebarOffset++
				}
			} else {
				m.viewport.LineDown(3)
			}

		case tea.MouseLeft:
			// 1. Click on bottom statusline bar buttons
			if msg.Y >= m.height-1 {
				cmd := m.handleBottomBarClick(msg.X)
				if cmd != nil {
					return m, cmd
				}
				return m, nil
			}

			// 2. Click in Sidebar area
			if msg.X < sidebarWidth && msg.Y >= 1 && msg.Y < m.height-1 {
				m.focusedPane = FocusSidebar
				m.input.Blur()

				// Click directly on a node row
				// Row 0: Top title bar
				// Row 1: Sidebar border top
				// Row 2: PROJECTS (N) header
				// Row 3: Top scroll indicator (if offset > 0) or empty line
				// Row 4: First visible node (offset + 0)
				if msg.Y == 3 && m.sidebarOffset > 0 {
					m.sidebarOffset--
					m.rebuildTree()
					return m, nil
				}
				if msg.Y >= 4 {
					nodeIdx := m.sidebarOffset + (msg.Y - 4)
					if nodeIdx >= 0 && nodeIdx < len(m.visibleNodes) {
						m.selectedNodeIdx = nodeIdx
						node := m.visibleNodes[nodeIdx]
						if node.IsHeader {
							m.expandedState[node.ProjectName] = !m.expandedState[node.ProjectName]
							m.rebuildTree()
						} else if node.Conversation != nil {
							m.activeCascadeID = node.Conversation.CascadeID
							m.steps = nil
							m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history..."))
							cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
							m.focusedPane = FocusInput
							m.input.Focus()
						}
					}
				}
				return m, tea.Batch(cmds...)
			}

			// 3. Click in Right pane area
			if msg.X >= sidebarWidth && msg.Y >= 1 && msg.Y < m.height-1 {
				inViewHeight := m.input.Height() + 2
				if msg.Y >= m.height-inViewHeight-1 {
					// Clicked on Input box
					m.focusedPane = FocusInput
					m.input.Focus()
				} else {
					// Clicked on Chat transcript
					m.focusedPane = FocusChat
					m.input.Blur()
				}
				return m, nil
			}
		}

	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)

	case periodicTickMsg:
		cmds = append(cmds, m.loadConversationsCmd(), m.periodicTickCmd())
		if m.activeCascadeID != "" {
			cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
		}

	case conversationsLoadedMsg:
		// Filter out any conversations deleted locally
		var merged []hub.ConversationItem
		seen := make(map[string]bool)

		for _, item := range msg.items {
			if m.deletedCascadeIDs[item.CascadeID] {
				continue
			}
			merged = append(merged, item)
			seen[item.CascadeID] = true
		}

		// Re-add any locally created conversations that the hub doesn't report yet
		for id, conv := range m.createdConversations {
			if !m.deletedCascadeIDs[id] && !seen[id] {
				merged = append([]hub.ConversationItem{conv}, merged...)
				seen[id] = true
			}
		}

		m.conversations = merged
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Connected (Port %d, %d projects, %d sessions)",
			m.hubInfo.HttpPort, len(m.projectGroups), len(m.conversations))

		if m.activeCascadeID == "" && len(m.conversations) > 0 {
			m.activeCascadeID = m.conversations[0].CascadeID
			m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history..."))
			cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
		}

	case stepsLoadedMsg:
		if msg.cascadeID == m.activeCascadeID {
			m.steps = msg.steps
			if len(m.steps) == 0 && m.createdConversations != nil && m.createdConversations[msg.cascadeID].CascadeID != "" {
				conv := m.createdConversations[msg.cascadeID]
				m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render(
					fmt.Sprintf("✨ New conversation created in '%s'.\n\nType your message below and press Enter to start working with the agent.", conv.WorkspaceName),
				))
			} else {
				m.viewport.SetContent(m.renderTranscript())
			}
			m.viewport.GotoBottom()
		}

	case conversationDeletedMsg:
		var remaining []hub.ConversationItem
		for _, c := range m.conversations {
			if c.CascadeID != msg.cascadeID {
				remaining = append(remaining, c)
			}
		}
		m.conversations = remaining
		if m.activeCascadeID == msg.cascadeID {
			if len(m.conversations) > 0 {
				m.activeCascadeID = m.conversations[0].CascadeID
				cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
			} else {
				m.activeCascadeID = ""
				m.steps = nil
				m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("No active conversations. Press Ctrl+N to create one."))
			}
		}
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Deleted conversation '%s'", msg.title)
		return m, tea.Batch(cmds...)

	case projectDeletedMsg:
		var remaining []hub.ConversationItem
		for _, c := range m.conversations {
			if c.WorkspaceName != msg.projectName {
				remaining = append(remaining, c)
			}
		}
		m.conversations = remaining
		delete(m.expandedState, msg.projectName)
		if len(m.conversations) > 0 {
			m.activeCascadeID = m.conversations[0].CascadeID
			cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
		} else {
			m.activeCascadeID = ""
			m.steps = nil
			m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("No active conversations. Press Ctrl+P to create a project."))
		}
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Deleted project '%s'", msg.projectName)
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg.err
		m.statusMsg = fmt.Sprintf("Error: %v", msg.err)

	case tea.KeyMsg:
		if m.modalMode == ModalNewProject {
			switch msg.String() {
			case "esc":
				m.modalMode = ModalNone
				m.modalInput = ""
				m.statusMsg = "Project creation cancelled."
				return m, nil
			case "enter":
				projName := strings.TrimSpace(m.modalInput)
				m.modalMode = ModalNone
				m.modalInput = ""
				if projName != "" {
					return m, m.createNewConversation(projName)
				}
				m.statusMsg = "Project name cannot be empty."
				return m, nil
			case "backspace":
				if len(m.modalInput) > 0 {
					m.modalInput = m.modalInput[:len(m.modalInput)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.modalInput += msg.String()
				}
				return m, nil
			}
		}

		if m.modalMode == ModalConfirmDelete {
			switch strings.ToLower(msg.String()) {
			case "y":
				m.modalMode = ModalNone
				return m, m.executeDeleteCmd()
			case "n", "esc":
				m.modalMode = ModalNone
				m.statusMsg = "Delete cancelled."
				return m, nil
			default:
				m.modalMode = ModalNone
				m.statusMsg = "Delete cancelled."
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c":
			if m.streamCancelFunc != nil {
				m.streamCancelFunc()
			}
			return m, tea.Quit

		case "ctrl+n":
			cmd := m.createNewConversation("")
			return m, cmd

		case "ctrl+p":
			m.startNewProjectPrompt()
			return m, nil

		case "ctrl+d", "ctrl+x", "delete":
			m.startDeleteChatPrompt()
			return m, nil

		case "alt+d", "alt+delete":
			m.startDeleteProjPrompt()
			return m, nil

		case "tab":
			m.cyclePaneFocus()
			return m, nil

		case "esc":
			if m.isStreaming {
				m.statusMsg = "Stopping agent..."
				cmds = append(cmds, m.cancelCascadeCmd())
			} else if m.focusedPane != FocusSidebar {
				m.focusedPane = FocusSidebar
				m.input.Blur()
			}
			return m, nil

		case "r":
			m.statusMsg = "Refreshing conversations..."
			cmds = append(cmds, m.loadConversationsCmd())
			if m.activeCascadeID != "" {
				cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
			}
			return m, tea.Batch(cmds...)
		}

		// Pane specific handling
		switch m.focusedPane {
		case FocusSidebar:
			switch msg.String() {
			// WASD + Arrow Keys + Vim keys for navigation
			case "up", "k", "w", "W":
				if m.selectedNodeIdx > 0 {
					m.selectedNodeIdx--
					m.rebuildTree()
				}
			case "down", "j", "s", "S":
				if m.selectedNodeIdx < len(m.visibleNodes)-1 {
					m.selectedNodeIdx++
					m.rebuildTree()
				}
			case "enter", " ":
				if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
					node := m.visibleNodes[m.selectedNodeIdx]
					if node.IsHeader {
						m.expandedState[node.ProjectName] = !m.expandedState[node.ProjectName]
						m.rebuildTree()
					} else if node.Conversation != nil {
						m.activeCascadeID = node.Conversation.CascadeID
						m.steps = nil
						m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history..."))
						cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
						m.focusedPane = FocusInput
						m.input.Focus()
					}
				}
			case "right", "l", "d", "D":
				if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
					node := m.visibleNodes[m.selectedNodeIdx]
					if node.IsHeader {
						if !m.expandedState[node.ProjectName] {
							m.expandedState[node.ProjectName] = true
							m.rebuildTree()
						}
					} else if node.Conversation != nil {
						m.activeCascadeID = node.Conversation.CascadeID
						m.steps = nil
						m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history..."))
						cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
						m.focusedPane = FocusInput
						m.input.Focus()
					}
				}
			case "left", "h", "a", "A":
				if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
					node := m.visibleNodes[m.selectedNodeIdx]
					if node.IsHeader && m.expandedState[node.ProjectName] {
						m.expandedState[node.ProjectName] = false
						m.rebuildTree()
					} else if !node.IsHeader {
						// Jump up to parent project header
						for i := m.selectedNodeIdx - 1; i >= 0; i-- {
							if m.visibleNodes[i].IsHeader && m.visibleNodes[i].ProjectName == node.ProjectName {
								m.selectedNodeIdx = i
								m.rebuildTree()
								break
							}
						}
					}
				}
			case "n":
				cmd := m.createNewConversation("")
				return m, cmd
			case "p":
				m.startNewProjectPrompt()
				return m, nil
			case "x":
				if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) && m.visibleNodes[m.selectedNodeIdx].IsHeader {
					m.startDeleteProjPrompt()
				} else {
					m.startDeleteChatPrompt()
				}
				return m, nil
			}

		case FocusInput:
			switch msg.Type {
			case tea.KeyPgUp:
				m.viewport.HalfViewUp()
				return m, nil
			case tea.KeyPgDown:
				m.viewport.HalfViewDown()
				return m, nil
			case tea.KeyEnter:
				if !msg.Alt {
					text := strings.TrimSpace(m.input.Value())
					if text != "" {
						m.input.Reset()
						m.statusMsg = "Sent message to agent..."
						cmds = append(cmds, m.sendMessageCmd(text))
						for i := range m.conversations {
							if m.conversations[i].CascadeID == m.activeCascadeID && m.conversations[i].Title == "New Conversation" {
								firstLine := strings.Split(text, "\n")[0]
								if len(firstLine) > 35 {
									firstLine = firstLine[:32] + "..."
								}
								m.conversations[i].Title = firstLine
								m.rebuildTree()
								break
							}
						}
						if m.activeCascadeID != "" {
							cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
						}
					}
					return m, tea.Batch(cmds...)
				}
			}

			if msg.String() == "ctrl+u" {
				m.viewport.HalfViewUp()
				return m, nil
			}
			if msg.String() == "ctrl+d" {
				m.viewport.HalfViewDown()
				return m, nil
			}

			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		case FocusChat:
			switch msg.String() {
			case "up", "k":
				m.viewport.LineUp(1)
				return m, nil
			case "down", "j":
				m.viewport.LineDown(1)
				return m, nil
			case "pgup":
				m.viewport.HalfViewUp()
				return m, nil
			case "pgdown":
				m.viewport.HalfViewDown()
				return m, nil
			case "home", "g":
				m.viewport.GotoTop()
				return m, nil
			case "end", "G":
				m.viewport.GotoBottom()
				return m, nil
			}

			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	sidebarWidth := 36
	contentWidth := m.width - sidebarWidth - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Dynamic input box height based on terminal height:
	// Height < 16: 1 line (total inView = 3 lines with borders)
	// Height < 24: 2 lines (total inView = 4 lines with borders)
	// Height >= 24: 3 lines (total inView = 5 lines with borders)
	inputHeight := 3
	switch {
	case m.height < 16:
		inputHeight = 1
	case m.height < 24:
		inputHeight = 2
	default:
		inputHeight = 3
	}
	m.input.SetHeight(inputHeight)

	bodyHeight := m.height - 2
	inViewHeight := inputHeight + 2
	// rightPane total = 1 (chatHeader) + (viewportHeight + 2) + inViewHeight == bodyHeight
	viewportHeight := bodyHeight - 1 - inViewHeight - 2
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	m.viewport.Width = contentWidth - 4
	m.viewport.Height = viewportHeight
	m.input.SetWidth(contentWidth - 4)

	m.rebuildTree()
	if len(m.steps) > 0 {
		m.viewport.SetContent(m.renderTranscript())
	}
}

func (m *Model) createNewConversation(projectName string) tea.Cmd {
	if projectName == "" {
		projectName = "General"
		if len(m.visibleNodes) > 0 && m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
			projectName = m.visibleNodes[m.selectedNodeIdx].ProjectName
		} else if m.activeCascadeID != "" {
			for _, c := range m.conversations {
				if c.CascadeID == m.activeCascadeID && c.WorkspaceName != "" {
					projectName = c.WorkspaceName
					break
				}
			}
		}
	}

	newID := hub.NewUUID()
	now := time.Now().UTC().Format(time.RFC3339)
	newConv := hub.ConversationItem{
		CascadeID:        newID,
		Title:            "New Conversation",
		WorkspaceName:    projectName,
		Status:           "CASCADE_RUN_STATUS_IDLE",
		LastModifiedTime: now,
	}

	if m.createdConversations == nil {
		m.createdConversations = make(map[string]hub.ConversationItem)
	}
	m.createdConversations[newID] = newConv
	delete(m.deletedCascadeIDs, newID)

	m.conversations = append([]hub.ConversationItem{newConv}, m.conversations...)
	m.activeCascadeID = newID
	m.steps = nil
	m.expandedState[projectName] = true
	m.rebuildTree()

	// Move cursor to the new conversation in sidebar
	for i, node := range m.visibleNodes {
		if !node.IsHeader && node.Conversation != nil && node.Conversation.CascadeID == newID {
			m.selectedNodeIdx = i
			break
		}
	}
	m.rebuildTree()

	m.focusedPane = FocusInput
	m.input.Reset()
	m.input.Focus()
	m.statusMsg = fmt.Sprintf("✨ Started new conversation in '%s'", projectName)
	m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render(
		fmt.Sprintf("✨ New conversation created in '%s'.\n\nType your message below and press Enter to start working with the agent.", projectName),
	))
	return nil
}

func (m *Model) startNewProjectPrompt() {
	m.modalMode = ModalNewProject
	m.modalInput = ""
	m.statusMsg = "📁 Create Project: Type project name and press Enter (Esc to cancel)"
}

func (m *Model) startDeleteChatPrompt() {
	var targetConv *hub.ConversationItem

	// 1. If currently highlighted node in sidebar is a conversation, target it
	if m.focusedPane == FocusSidebar && m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
		node := m.visibleNodes[m.selectedNodeIdx]
		if !node.IsHeader && node.Conversation != nil {
			targetConv = node.Conversation
		}
	}

	// 2. Otherwise target the currently active/open conversation
	if targetConv == nil && m.activeCascadeID != "" {
		for i := range m.conversations {
			if m.conversations[i].CascadeID == m.activeCascadeID {
				targetConv = &m.conversations[i]
				break
			}
		}
	}

	if targetConv == nil {
		m.statusMsg = "No conversation selected to delete."
		return
	}

	m.modalMode = ModalConfirmDelete
	m.deleteIsProject = false
	m.deleteTargetID = targetConv.CascadeID
	m.deleteTargetName = targetConv.Title
	if m.deleteTargetName == "" {
		m.deleteTargetName = targetConv.CascadeID
	}
	m.statusMsg = fmt.Sprintf("⚠️ Delete conversation '%s'? (y/n)", m.deleteTargetName)
}

func (m *Model) startDeleteProjPrompt() {
	targetProj := ""

	// 1. If currently highlighted node in sidebar has a project name, target it
	if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
		targetProj = m.visibleNodes[m.selectedNodeIdx].ProjectName
	}

	// 2. Otherwise target the project of the active conversation
	if targetProj == "" && m.activeCascadeID != "" {
		for i := range m.conversations {
			if m.conversations[i].CascadeID == m.activeCascadeID {
				targetProj = m.conversations[i].WorkspaceName
				break
			}
		}
	}

	if targetProj == "" {
		m.statusMsg = "No project selected to delete."
		return
	}

	m.modalMode = ModalConfirmDelete
	m.deleteIsProject = true
	m.deleteTargetName = targetProj
	m.deleteTargetID = ""
	m.statusMsg = fmt.Sprintf("⚠️ Delete project '%s' and all its chats? (y/n)", targetProj)
}

func (m *Model) executeDeleteCmd() tea.Cmd {
	isProj := m.deleteIsProject
	targetID := m.deleteTargetID
	targetName := m.deleteTargetName

	if m.deletedCascadeIDs == nil {
		m.deletedCascadeIDs = make(map[string]bool)
	}

	if isProj {
		for _, conv := range m.conversations {
			if conv.WorkspaceName == targetName {
				m.deletedCascadeIDs[conv.CascadeID] = true
				delete(m.createdConversations, conv.CascadeID)
			}
		}
		var remaining []hub.ConversationItem
		for _, conv := range m.conversations {
			if conv.WorkspaceName != targetName {
				remaining = append(remaining, conv)
			}
		}
		m.conversations = remaining
		delete(m.expandedState, targetName)
		if len(m.conversations) > 0 {
			m.activeCascadeID = m.conversations[0].CascadeID
		} else {
			m.activeCascadeID = ""
			m.steps = nil
			m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("No active conversations. Press Ctrl+P to create a project."))
		}
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Deleted project '%s'", targetName)
	} else {
		m.deletedCascadeIDs[targetID] = true
		delete(m.createdConversations, targetID)
		var remaining []hub.ConversationItem
		for _, conv := range m.conversations {
			if conv.CascadeID != targetID {
				remaining = append(remaining, conv)
			}
		}
		m.conversations = remaining
		if m.activeCascadeID == targetID {
			if len(m.conversations) > 0 {
				m.activeCascadeID = m.conversations[0].CascadeID
			} else {
				m.activeCascadeID = ""
				m.steps = nil
				m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("No active conversations. Press Ctrl+N to create one."))
			}
		}
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Deleted conversation '%s'", targetName)
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if isProj {
			for _, conv := range m.conversations {
				if conv.WorkspaceName == targetName {
					_ = m.client.DeleteCascade(ctx, conv.CascadeID)
				}
			}
			return projectDeletedMsg{projectName: targetName}
		}

		_ = m.client.DeleteCascade(ctx, targetID)
		return conversationDeletedMsg{cascadeID: targetID, title: targetName}
	}
}

func (m *Model) handleBottomBarClick(x int) tea.Cmd {
	_, hitboxes := m.renderButtonBar()
	for _, btn := range hitboxes {
		if x >= btn.MinX && x <= btn.MaxX {
			return m.triggerAction(btn.ID)
		}
	}
	return nil
}

func (m *Model) triggerAction(actionID string) tea.Cmd {
	var cmds []tea.Cmd
	switch actionID {
	case "confirm_delete":
		m.modalMode = ModalNone
		return m.executeDeleteCmd()
	case "cancel_delete":
		m.modalMode = ModalNone
		m.statusMsg = "Delete cancelled."
		return nil
	case "create_proj":
		projName := strings.TrimSpace(m.modalInput)
		m.modalMode = ModalNone
		m.modalInput = ""
		if projName != "" {
			return m.createNewConversation(projName)
		}
		m.statusMsg = "Project name cannot be empty."
		return nil
	case "cancel_proj":
		m.modalMode = ModalNone
		m.modalInput = ""
		m.statusMsg = "Project creation cancelled."
		return nil
	case "tab":
		m.cyclePaneFocus()
	case "open":
		if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
			node := m.visibleNodes[m.selectedNodeIdx]
			if node.IsHeader {
				m.expandedState[node.ProjectName] = !m.expandedState[node.ProjectName]
				m.rebuildTree()
			} else if node.Conversation != nil {
				m.activeCascadeID = node.Conversation.CascadeID
				m.steps = nil
				m.viewport.SetContent(lipgloss.NewStyle().Foreground(subduedColor).Render("Loading conversation history..."))
				cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
				m.focusedPane = FocusInput
				m.input.Focus()
			}
		}
	case "new_chat":
		cmd := m.createNewConversation("")
		return cmd
	case "del_chat":
		m.startDeleteChatPrompt()
	case "new_proj":
		m.startNewProjectPrompt()
	case "del_proj":
		m.startDeleteProjPrompt()
	case "refresh":
		m.statusMsg = "Refreshing conversations..."
		cmds = append(cmds, m.loadConversationsCmd())
		if m.activeCascadeID != "" {
			cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
		}
	case "quit":
		if m.streamCancelFunc != nil {
			m.streamCancelFunc()
		}
		return tea.Quit
	}
	return tea.Batch(cmds...)
}
