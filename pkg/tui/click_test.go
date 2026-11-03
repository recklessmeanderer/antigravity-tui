package tui

import (
	"antigravity-tui/pkg/hub"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestMouseClickRegistration(t *testing.T) {
	convs := []hub.ConversationItem{
		{CascadeID: "c1", Title: "Task 1", WorkspaceName: "ProjectA"},
		{CascadeID: "c2", Title: "Task 2", WorkspaceName: "ProjectB"},
		{CascadeID: "c3", Title: "Task 3", WorkspaceName: "ProjectC"},
	}

	h := 13
	m := NewModel(nil, &hub.HubInfo{HttpPort: 36919, Pid: 1234})
	m.width = 152
	m.height = h
	m.recalculateLayout()

	updated, _ := m.Update(conversationsLoadedMsg{items: convs})
	m = updated.(Model)

	// Click on ProjectB at Y=5 (X=5)
	updated, _ = m.Update(tea.MouseMsg{
		X: 5,
		Y: 5,
		Type: tea.MouseLeft,
	})
	m = updated.(Model)
	t.Logf("After click Y=5: selectedNodeIdx=%d expandedState=%v", m.selectedNodeIdx, m.expandedState)

	// Now click on ProjectA at Y=4 (X=5)
	updated, _ = m.Update(tea.MouseMsg{
		X: 5,
		Y: 4,
		Type: tea.MouseLeft,
	})
	m = updated.(Model)
	t.Logf("After click Y=4: selectedNodeIdx=%d expandedState=%v", m.selectedNodeIdx, m.expandedState)
}
