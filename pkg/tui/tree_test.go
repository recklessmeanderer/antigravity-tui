package tui

import (
	"testing"

	"antigravity-tui/pkg/hub"
)

func TestBuildProjectGroups(t *testing.T) {
	convs := []hub.ConversationItem{
		{
			CascadeID:        "c1",
			Title:            "Task 1",
			WorkspaceName:    "antigravity-tui",
			LastModifiedTime: "2026-11-03T01:00:00Z",
		},
		{
			CascadeID:        "c2",
			Title:            "Task 2",
			WorkspaceName:    "antigravity-tui",
			LastModifiedTime: "2026-11-03T02:00:00Z",
		},
		{
			CascadeID:        "c3",
			Title:            "Task 3",
			WorkspaceName:    "oscillopulse",
			LastModifiedTime: "2026-11-03T03:00:00Z",
		},
	}

	expandedState := make(map[string]bool)

	// Test 1: All collapsed by default
	groups, nodes := BuildProjectGroupsWithProjects(convs, expandedState, nil)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	// Newer group "oscillopulse" should be first
	if groups[0].Name != "oscillopulse" {
		t.Errorf("expected first group 'oscillopulse', got %s", groups[0].Name)
	}
	// When collapsed, only 2 header nodes should exist
	if len(nodes) != 2 {
		t.Fatalf("expected 2 visible nodes when collapsed, got %d", len(nodes))
	}

	// Test 2: Expand "antigravity-tui"
	expandedState["antigravity-tui"] = true
	_, nodes = BuildProjectGroupsWithProjects(convs, expandedState, nil)
	// 2 headers + 2 conversations under antigravity-tui = 4 nodes
	if len(nodes) != 4 {
		t.Fatalf("expected 4 visible nodes when one is expanded, got %d", len(nodes))
	}
	if !nodes[1].IsHeader || nodes[1].ProjectName != "antigravity-tui" {
		t.Errorf("expected node 1 to be header for antigravity-tui, got %+v", nodes[1])
	}
	if nodes[2].IsHeader || nodes[2].Conversation.CascadeID != "c2" {
		t.Errorf("expected node 2 to be conversation c2 (newest), got %+v", nodes[2])
	}

	// Test 3: Known projects included even with zero conversations
	knownProjects := []string{"investments", "oscillopulse", "antigravity-tui"}
	groups3, _ := BuildProjectGroupsWithProjects(convs, expandedState, knownProjects)
	if len(groups3) != 3 {
		t.Fatalf("expected 3 groups including investments, got %d", len(groups3))
	}
	foundInvestments := false
	for _, g := range groups3 {
		if g.Name == "investments" {
			foundInvestments = true
			if len(g.Conversations) != 0 {
				t.Errorf("expected 0 conversations for investments, got %d", len(g.Conversations))
			}
		}
	}
	if !foundInvestments {
		t.Errorf("expected investments group to be present in groups3")
	}
}
