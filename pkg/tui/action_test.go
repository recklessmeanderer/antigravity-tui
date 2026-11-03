package tui

import (
	"strings"
	"testing"
	"antigravity-tui/pkg/hub"
	tea "github.com/charmbracelet/bubbletea"
)

func TestActionsKeyboardAndMouse(t *testing.T) {
	convs := []hub.ConversationItem{
		{CascadeID: "conv-1", Title: "Old Chat", WorkspaceName: "ProjectA"},
	}

	m := NewModel(nil, &hub.HubInfo{HttpPort: 36919, Pid: 1234})
	m.width = 152
	m.height = 24
	m.recalculateLayout()

	updated, _ := m.Update(conversationsLoadedMsg{items: convs})
	m = updated.(Model)

	// 1. Test New Conversation (Ctrl+N)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = updated.(Model)
	if len(m.conversations) != 2 {
		t.Fatalf("Expected 2 conversations after Ctrl+N, got %d", len(m.conversations))
	}
	if !strings.Contains(m.statusMsg, "Started new conversation") {
		t.Fatalf("Unexpected statusMsg: %s", m.statusMsg)
	}

	// Verify periodic tick / conversationsLoadedMsg does NOT wipe out the new conversation
	updated, _ = m.Update(conversationsLoadedMsg{items: convs})
	m = updated.(Model)
	if len(m.conversations) != 2 {
		t.Fatalf("Expected 2 conversations preserved across hub sync, got %d", len(m.conversations))
	}

	// 2. Test New Project Prompt (Ctrl+P)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	if m.modalMode != ModalNewProject {
		t.Fatalf("Expected modalMode ModalNewProject, got %v", m.modalMode)
	}
	// Type project name: "cool-app"
	for _, ch := range "cool-app" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = updated.(Model)
	}
	if m.modalInput != "cool-app" {
		t.Fatalf("Expected modalInput 'cool-app', got '%s'", m.modalInput)
	}
	// Press Enter to submit
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.modalMode != ModalNone {
		t.Fatalf("Expected modalMode ModalNone after submit, got %v", m.modalMode)
	}
	if len(m.conversations) != 3 {
		t.Fatalf("Expected 3 conversations after project create, got %d", len(m.conversations))
	}
	if m.conversations[0].WorkspaceName != "cool-app" {
		t.Fatalf("Expected first conversation workspace 'cool-app', got '%s'", m.conversations[0].WorkspaceName)
	}

	// Verify new project is preserved across background sync
	updated, _ = m.Update(conversationsLoadedMsg{items: convs})
	m = updated.(Model)
	if len(m.conversations) != 3 {
		t.Fatalf("Expected 3 conversations preserved after second hub sync, got %d", len(m.conversations))
	}

	// 3. Test Delete Chat Prompt (Ctrl+D)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = updated.(Model)
	if m.modalMode != ModalConfirmDelete {
		t.Fatalf("Expected modalMode ModalConfirmDelete, got %v", m.modalMode)
	}
	if m.deleteIsProject {
		t.Fatalf("Expected deleteIsProject=false for Delete Chat, got true")
	}
	// Cancel with 'n'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	if m.modalMode != ModalNone {
		t.Fatalf("Expected modalMode ModalNone after cancel, got %v", m.modalMode)
	}

	// 4. Test Mouse Click on "🗑 Del Chat (Ctrl+D)" button
	_, hitboxes := m.renderButtonBar()
	var delChatBtn *ButtonHitbox
	for i := range hitboxes {
		if hitboxes[i].ID == "del_chat" {
			delChatBtn = &hitboxes[i]
			break
		}
	}
	if delChatBtn == nil {
		t.Fatalf("Could not find del_chat button in hitboxes")
	}
	clickX := (delChatBtn.MinX + delChatBtn.MaxX) / 2
	updated, _ = m.Update(tea.MouseMsg{
		X: clickX,
		Y: m.height - 1,
		Type: tea.MouseLeft,
	})
	m = updated.(Model)
	if m.modalMode != ModalConfirmDelete {
		t.Fatalf("Expected ModalConfirmDelete after clicking del_chat, got %v", m.modalMode)
	}
}
