package hub

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type projectConfigFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LoadProjectMap returns a map of project ID -> human-readable project Name
// discovered from ~/.gemini/config/projects/*.json.
func LoadProjectMap() map[string]string {
	m := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return m
	}

	pattern := filepath.Join(home, ".gemini", "config", "projects", "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		return m
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var p projectConfigFile
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.ID == "" || p.Name == "" {
			continue
		}
		// Skip plugin-internal projects like "android-cli-plugin/compose-preview"
		if strings.Contains(p.Name, "/") {
			continue
		}
		m[p.ID] = p.Name
	}
	return m
}

// LoadConversationProjectIDs queries ~/.gemini/antigravity/conversation_summaries.db
// to return a map of conversation ID -> project ID.
func LoadConversationProjectIDs() map[string]string {
	result := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return result
	}

	dbPath := filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return result
	}

	cmd := exec.Command("sqlite3", "-init", "/dev/null", "-csv", dbPath,
		"SELECT conversation_id, project_id FROM conversation_summaries WHERE project_id != '' AND project_id IS NOT NULL;")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	reader := csv.NewReader(strings.NewReader(string(output)))
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 {
		return result
	}

	for _, row := range rows {
		if len(row) >= 2 {
			cid := strings.TrimSpace(row[0])
			pid := strings.TrimSpace(row[1])
			if cid != "" && pid != "" {
				result[cid] = pid
			}
		}
	}
	return result
}

// LoadAllProjectNames returns a sorted list of all unique project names configured in Antigravity.
func LoadAllProjectNames() []string {
	pm := LoadProjectMap()
	seen := make(map[string]bool)
	var names []string

	for _, name := range pm {
		clean := strings.TrimSpace(name)
		if clean == "" || clean == "outside-of-project" || clean == "default-cli-project" {
			continue
		}
		if !seen[clean] {
			seen[clean] = true
			names = append(names, clean)
		}
	}
	sort.Strings(names)
	return names
}
