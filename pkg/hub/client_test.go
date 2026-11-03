package hub

import (
	"context"
	"testing"
	"time"
)

func TestLiveHubConnection(t *testing.T) {
	hubInfo, err := DiscoverHub("")
	if err != nil {
		t.Skipf("Skipping live hub test (no active hub found): %v", err)
	}

	client, err := NewClient(hubInfo)
	if err != nil {
		t.Fatalf("Failed to connect to live hub: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conversations, err := client.ListAllConversations(ctx)
	if err != nil {
		t.Fatalf("ListAllConversations failed: %v", err)
	}

	if len(conversations) == 0 {
		t.Fatalf("Expected non-empty conversations list")
	}

	t.Logf("Live hub connected successfully! Found %d total conversations across workspaces.", len(conversations))
	for i, c := range conversations {
		if i < 5 {
			t.Logf(" - [%d] ID: %s | Title: %s | Status: %s", i, c.CascadeID, c.Title, c.Status)
		}
	}
}
