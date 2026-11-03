package hub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverHubWithOverride(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Valid override path exists and has valid fields", func(t *testing.T) {
		hubContent := `{"pid": 12345, "httpsPort": 443, "httpPort": 36919, "lsVersion": "1.0", "csrfToken": "abc123"}`
		filePath := filepath.Join(tempDir, "hub_override.json")
		if err := os.WriteFile(filePath, []byte(hubContent), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		result, err := DiscoverHub(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Pid != 12345 || result.HttpPort != 36919 || result.CsrfToken != "abc123" {
			t.Errorf("unexpected hub info fields: %+v", result)
		}
		if result.ConfigPath != filePath {
			t.Errorf("expected config path %s, got %s", filePath, result.ConfigPath)
		}
	})

	t.Run("Valid override path exists but missing httpPort", func(t *testing.T) {
		hubContent := `{"pid": 12345, "httpsPort": 443, "lsVersion": "1.0", "csrfToken": "abc123"}`
		filePath := filepath.Join(tempDir, "hub_no_port.json")
		if err := os.WriteFile(filePath, []byte(hubContent), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		result, err := DiscoverHub(filePath)
		if err == nil {
			t.Errorf("expected error for missing httpPort, got result: %+v", result)
		}
	})

	t.Run("Valid override path exists but missing csrfToken", func(t *testing.T) {
		hubContent := `{"pid": 12345, "httpsPort": 443, "httpPort": 36919, "lsVersion": "1.0"}`
		filePath := filepath.Join(tempDir, "hub_no_token.json")
		if err := os.WriteFile(filePath, []byte(hubContent), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		result, err := DiscoverHub(filePath)
		if err == nil {
			t.Errorf("expected error for missing csrfToken, got result: %+v", result)
		}
	})

	t.Run("Invalid override path returns error", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent.json")
		result, err := DiscoverHub(filePath)
		if err == nil {
			t.Errorf("expected error for nonexistent file, got result: %+v", result)
		}
	})
}
