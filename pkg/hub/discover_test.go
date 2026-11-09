package hub

import (
	"fmt"
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

func TestParsePortFromLog(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Extracts latest port correctly", func(t *testing.T) {
		logContent := `I1108 18:51:39.230832 Starting language server process with pid 6587
I1108 18:51:39.232966 Language server listening on random port at 36129 for HTTPS (gRPC)
I1108 18:51:39.233405 Language server listening on random port at 35825 for HTTP
`
		logPath := filepath.Join(tempDir, "test.log")
		if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
			t.Fatalf("failed to write test log: %v", err)
		}

		port := parsePortFromLog(logPath)
		if port != 35825 {
			t.Errorf("expected port 35825, got %d", port)
		}
	})

	t.Run("Returns 0 for nonexistent log", func(t *testing.T) {
		port := parsePortFromLog(filepath.Join(tempDir, "missing.log"))
		if port != 0 {
			t.Errorf("expected 0, got %d", port)
		}
	})
}

func TestIsPidAlive(t *testing.T) {
	myPid := os.Getpid()
	if !isPidAlive(myPid) {
		t.Errorf("expected current process PID %d to be alive", myPid)
	}

	deadPid := 99999999
	if isPidAlive(deadPid) {
		t.Errorf("expected PID %d to be reported dead", deadPid)
	}
}

func TestSaveAndCleanDaemonFiles(t *testing.T) {
	tempDir := t.TempDir()

	hub := &HubInfo{
		Pid:       os.Getpid(),
		HttpPort:  55555,
		CsrfToken: "test-token",
		LsVersion: "2.28.2",
	}

	if err := saveHubInfo(tempDir, hub); err != nil {
		t.Fatalf("saveHubInfo failed: %v", err)
	}

	dest := filepath.Join(tempDir, fmt.Sprintf("hub_%d.json", os.Getpid()))
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected hub json file at %s: %v", dest, err)
	}

	// Create a fake dead pid file
	deadFile := filepath.Join(tempDir, "hub_99999998.json")
	deadContent := `{"pid": 99999998, "httpPort": 1234, "csrfToken": "fake"}`
	if err := os.WriteFile(deadFile, []byte(deadContent), 0644); err != nil {
		t.Fatalf("failed to write dead file: %v", err)
	}

	cleanStaleDaemonFiles(tempDir, os.Getpid())

	// Dead file should be cleaned up
	if _, err := os.Stat(deadFile); !os.IsNotExist(err) {
		t.Errorf("expected dead file %s to be removed", deadFile)
	}

	// Live file should remain
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected live file %s to remain: %v", dest, err)
	}
}
