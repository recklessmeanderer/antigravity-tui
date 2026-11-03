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
	selectedIdx      int
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
		client:        client,
		hubInfo:       hubInfo,
		conversations: nil,
		selectedIdx:   0,
		viewport:      vp,
		input:         ta,
		spinner:       sp,
		focusedPane:   FocusInput,
		statusMsg:     "Connected to Antigravity Hub",
		lastUpdate:    time.Now(),
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
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()

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
		m.statusMsg = fmt.Sprintf("Connected (Port %d, %d sessions)", m.hubInfo.HttpPort, len(m.conversations))
		if len(m.conversations) > 0 && m.activeCascadeID == "" {
			m.activeCascadeID = m.conversations[0].CascadeID
			m.selectedIdx = 0
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
			// Cycle pane focus: Sidebar -> Input -> Chat -> Sidebar
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
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.activeCascadeID = m.conversations[m.selectedIdx].CascadeID
					cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
				}
			case "down", "j":
				if m.selectedIdx < len(m.conversations)-1 {
					m.selectedIdx++
					m.activeCascadeID = m.conversations[m.selectedIdx].CascadeID
					cmds = append(cmds, m.loadStepsCmd(m.activeCascadeID), m.startStreamingUpdates(m.activeCascadeID))
				}
			case "enter":
				m.focusedPane = FocusInput
				m.input.Focus()
			}

		case FocusInput:
			if msg.Type == tea.KeyEnter && !msg.Alt {
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
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		case FocusChat:
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

	m.viewport.SetContent(m.renderTranscript())
}
