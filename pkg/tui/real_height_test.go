package tui

import (
	"fmt"
	"strings"
	"testing"
	"antigravity-tui/pkg/hub"
)

func TestRealHeightSmall(t *testing.T) {
	var convs []hub.ConversationItem
	for i := 0; i < 37; i++ {
		convs = append(convs, hub.ConversationItem{
			CascadeID:     fmt.Sprintf("id-%d", i),
			Title:         fmt.Sprintf("Title %d", i),
			WorkspaceName: fmt.Sprintf("Project-%d", i%15),
		})
	}

	for h := 9; h <= 75; h++ {
		m := NewModel(nil, &hub.HubInfo{HttpPort: 36919, Pid: 866887})
		m.width = 152
		if h >= 70 {
			m.width = 315
		}
		m.height = h
		m.recalculateLayout()
		updated, _ := m.Update(conversationsLoadedMsg{items: convs})
		m = updated.(Model)
		m.recalculateLayout()
		lines := strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Fatalf("FAILED at height %d: got %d lines, want %d", h, len(lines), h)
		}
	}
}
