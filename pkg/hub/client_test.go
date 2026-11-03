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

	trajectories, err := client.GetAllTrajectories(ctx)
	if err != nil {
		t.Fatalf("GetAllTrajectories failed: %v", err)
	}

	t.Logf("Live hub connected successfully! Found %d trajectories.", len(trajectories))
	for id, sum := range trajectories {
		t.Logf(" - Trajectory ID %s: %s (steps: %d, status: %s)", id, sum.Summary, sum.StepCount, sum.Status)
	}
}
