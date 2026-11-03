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
}

// Msg types
type conversationsLoadedMsg struct {
	items []hub.ConversationItem
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
		spinner:         sp,
		focusedPane:     FocusInput,
		statusMsg:       "Connected to Antigravity Hub",
		lastUpdate:      time.Now(),
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
	maxVisible := m.height - 6
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
	inputHeight := 4

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
				// Widths of buttons in bottom statusline:
				// [Tab: Pane] ~ 0..14
				// [↵ Open/Expand] ~ 15..32
				// [▲ PgUp] ~ 33..43
				// [▼ PgDn] ~ 44..54
				// [⟳ Refresh] ~ 55..68
				// [✕ Quit] ~ 69..79
				switch {
				case msg.X >= 0 && msg.X <= 14:
					m.cyclePaneFocus()
					return m, nil
				case msg.X >= 15 && msg.X <= 32:
					// Open or expand current selection
					if m.selectedNodeIdx >= 0 && m.selectedNodeIdx < len(m.visibleNodes) {
						node := m.visibleNodes[m.selectedNodeIdx]
						if node.IsHeader {
							m.expandedState[node.ProjectName] = !m.expandedState[node.ProjectName]
							m.rebuildTree()
						} else if node.Conversation != nil {
							m.activeCascadeID = node.Conversation.CascadeID
							cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
							m.focusedPane = FocusInput
							m.input.Focus()
						}
					}
					return m, tea.Batch(cmds...)
				case msg.X >= 33 && msg.X <= 43:
					m.viewport.HalfViewUp()
					return m, nil
				case msg.X >= 44 && msg.X <= 54:
					m.viewport.HalfViewDown()
					return m, nil
				case msg.X >= 55 && msg.X <= 68:
					m.statusMsg = "Refreshing conversations..."
					cmds = append(cmds, m.loadConversationsCmd())
					if m.activeCascadeID != "" {
						cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID))
					}
					return m, tea.Batch(cmds...)
				case msg.X >= 69 && msg.X <= 82:
					if m.streamCancelFunc != nil {
						m.streamCancelFunc()
					}
					return m, tea.Quit
				}
			}

			// 2. Click in Sidebar area
			if msg.X < sidebarWidth && msg.Y >= 1 && msg.Y < m.height-1 {
				m.focusedPane = FocusSidebar
				m.input.Blur()

				// Click directly on a node row
				// Row 1 is header, row 2 is "PROJECTS (N)", row 3 is scroll indicator / margin
				if msg.Y >= 3 {
					nodeIdx := m.sidebarOffset + (msg.Y - 3)
					if nodeIdx >= 0 && nodeIdx < len(m.visibleNodes) {
						m.selectedNodeIdx = nodeIdx
						node := m.visibleNodes[nodeIdx]
						if node.IsHeader {
							m.expandedState[node.ProjectName] = !m.expandedState[node.ProjectName]
							m.rebuildTree()
						} else if node.Conversation != nil {
							m.activeCascadeID = node.Conversation.CascadeID
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
				if msg.Y >= m.height-inputHeight-1 {
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
		m.conversations = msg.items
		m.rebuildTree()
		m.statusMsg = fmt.Sprintf("Connected (Port %d, %d projects, %d sessions)",
			m.hubInfo.HttpPort, len(m.projectGroups), len(m.conversations))

		if m.activeCascadeID == "" && len(m.conversations) > 0 {
			m.activeCascadeID = m.conversations[0].CascadeID
			cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
		}

	case stepsLoadedMsg:
		if msg.cascadeID == m.activeCascadeID {
			m.steps = msg.steps
			m.viewport.SetContent(m.renderTranscript())
			m.viewport.GotoBottom()
		}

	case errMsg:
		m.err = msg.err
		m.statusMsg = fmt.Sprintf("Error: %v", msg.err)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.streamCancelFunc != nil {
				m.streamCancelFunc()
			}
			return m, tea.Quit

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

	inputHeight := 4
	viewportHeight := m.height - inputHeight - 5
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.viewport.Width = contentWidth - 4
	m.viewport.Height = viewportHeight
	m.input.SetWidth(contentWidth - 4)

	m.rebuildTree()
	m.viewport.SetContent(m.renderTranscript())
}
