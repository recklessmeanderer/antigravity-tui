package tui

import (
	"antigravity-tui/pkg/hub"
	"strings"
	"testing"
)

func TestLayoutHeights(t *testing.T) {
	convs := []hub.ConversationItem{
		{CascadeID: "test-1", Title: "Task 1", WorkspaceName: "proj1"},
		{CascadeID: "test-2", Title: "Task 2", WorkspaceName: "proj1"},
		{CascadeID: "test-3", Title: "Task 3", WorkspaceName: "proj2"},
	}

	for _, h := range []int{10, 13, 14, 16, 20, 24, 30, 70, 100} {
		m := NewModel(nil, &hub.HubInfo{HttpPort: 36919, Pid: 1234})
		m.width = 152
		if h >= 70 {
			m.width = 315
		}
		m.height = h
		m.recalculateLayout()

		// Test initial empty
		lines := strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Errorf("Empty at height %d: got %d lines, want %d", h, len(lines), h)
		}

		// Test with conversations loaded
		updated, _ := m.Update(conversationsLoadedMsg{items: convs})
		m = updated.(Model)
		lines = strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Errorf("Loaded at height %d: got %d lines, want %d", h, len(lines), h)
		}
	}
}
