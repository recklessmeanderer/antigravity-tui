package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"antigravity-tui/pkg/hub"
)

func TestLiveRenderTranscript(t *testing.T) {
	hubInfo, err := hub.DiscoverHub("")
	if err != nil || hubInfo == nil {
		t.Skip("No hub found")
	}

	client, err := hub.NewClient(hubInfo)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conversations, err := client.ListAllConversations(ctx)
	if err != nil || len(conversations) == 0 {
		t.Skip("No conversations found")
	}

	firstID := conversations[0].CascadeID
	steps, err := client.GetTrajectorySteps(ctx, firstID)
	if err != nil {
		t.Fatalf("GetTrajectorySteps failed: %v", err)
	}

	m := NewModel(client, hubInfo)
	m.steps = steps
	transcript := m.renderTranscript()

	m.viewport.Width = 80
	m.viewport.Height = 20
	m.viewport.SetContent(transcript)
	m.viewport.GotoBottom()
	t.Logf("Viewport View:\n%s", m.viewport.View())
	t.Logf("Viewport YOffset: %d, TotalLines: %d", m.viewport.YOffset, m.viewport.TotalLineCount())
	if len(transcript) < 100 {
		t.Errorf("Transcript surprisingly short: %q", transcript)
	}

	if !strings.Contains(transcript, "USER") {
		t.Errorf("Transcript does not contain USER section")
	}
}

func TestLaunchView(t *testing.T) {
	hubInfo, err := hub.DiscoverHub("")
	if err != nil || hubInfo == nil {
		t.Skip("No hub")
	}
	client, err := hub.NewClient(hubInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	m := NewModel(client, hubInfo)
	m.width = 120
	m.height = 30
	m.recalculateLayout()

	t.Logf("Initial View:\n%s", m.View())

	// Now simulate conversationsLoadedMsg
	ctx := context.Background()
	convs, err := client.ListAllConversations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	updatedModel, cmd := m.Update(conversationsLoadedMsg{items: convs})
	m = updatedModel.(Model)
	t.Logf("View after conversationsLoadedMsg:\n%s", m.View())
	_ = cmd
}
