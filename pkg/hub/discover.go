package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type HubInfo struct {
	Pid        int    `json:"pid"`
	HttpsPort  int    `json:"httpsPort"`
	HttpPort   int    `json:"httpPort"`
	LsVersion  string `json:"lsVersion"`
	CsrfToken  string `json:"csrfToken"`
	ConfigPath string `json:"-"`
}

// DiscoverHub locates and reads the active Antigravity hub configuration.
// If overridePath is non-empty, it reads that path directly; otherwise it scans
// ~/.gemini/antigravity/daemon/hub_*.json and selects the newest file.
func DiscoverHub(overridePath string) (*HubInfo, error) {
	if overridePath != "" {
		return readAndParseHub(overridePath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}

	daemonDir := filepath.Join(home, ".gemini/antigravity/daemon")
	files, err := filepath.Glob(filepath.Join(daemonDir, "hub_*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan hub files in %s: %w", daemonDir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no active Antigravity hub found in %s (make sure Antigravity is running)", daemonDir)
	}

	newestFile := ""
	newestTime := time.Time{}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestFile = file
		}
	}

	if newestFile == "" {
		return nil, fmt.Errorf("could not stat any hub file in %s", daemonDir)
	}
	return readAndParseHub(newestFile)
}

func readAndParseHub(filePath string) (*HubInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var hub HubInfo
	if err := json.Unmarshal(data, &hub); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in %s: %w", filePath, err)
	}
	hub.ConfigPath = filePath
	if hub.HttpPort == 0 || hub.CsrfToken == "" {
		return nil, fmt.Errorf("invalid HubInfo in %s (missing httpPort or csrfToken)", filePath)
	}
	return &hub, nil
}
