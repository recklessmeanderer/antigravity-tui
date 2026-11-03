package tui

import (
	"fmt"
	"testing"
	"antigravity-tui/pkg/hub"
	tea "github.com/charmbracelet/bubbletea"
)

func TestMouseClickWithManyProjects(t *testing.T) {
	var convs []hub.ConversationItem
	for i := 0; i < 37; i++ {
		convs = append(convs, hub.ConversationItem{
			CascadeID:     fmt.Sprintf("id-%d", i),
			Title:         fmt.Sprintf("Title %d", i),
			WorkspaceName: fmt.Sprintf("Project-%02d", i%15),
		})
	}

	h := 13
	m := NewModel(nil, &hub.HubInfo{HttpPort: 36919, Pid: 866887})
	m.width = 152
	m.height = h
	m.recalculateLayout()
	updated, _ := m.Update(conversationsLoadedMsg{items: convs})
	m = updated.(Model)
	m.recalculateLayout()

	// Click Row 4 -> should toggle topProject
	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: 4, Type: tea.MouseLeft})
	m = updated.(Model)

	t.Logf("After click 1: sidebarOffset=%d selectedNodeIdx=%d", m.sidebarOffset, m.selectedNodeIdx)
	for i := 0; i < len(m.visibleNodes) && i < 10; i++ {
		t.Logf("  [%2d] isHeader=%v proj=%s", i, m.visibleNodes[i].IsHeader, m.visibleNodes[i].ProjectName)
	}

	// Click Row 4 again
	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: 4, Type: tea.MouseLeft})
	m = updated.(Model)
	t.Logf("After click 2: sidebarOffset=%d selectedNodeIdx=%d", m.sidebarOffset, m.selectedNodeIdx)
}
