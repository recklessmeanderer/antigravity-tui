package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var pinsMu sync.Mutex

// LoadPinnedConversations returns the ordered list of pinned conversation IDs
// and a lookup set. It loads from ~/.config/Antigravity/app_storage.json and scans
// ~/.gemini/antigravity/annotations/*.pbtxt.
func LoadPinnedConversations() ([]string, map[string]bool) {
	pinsMu.Lock()
	defer pinsMu.Unlock()

	var ordered []string
	seen := make(map[string]bool)

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ordered, seen
	}

	// 1. Primary: read app_storage.json
	storagePath := filepath.Join(home, ".config", "Antigravity", "app_storage.json")
	if data, err := os.ReadFile(storagePath); err == nil {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if orderRaw, ok := rawMap["pinned_conversations_order"]; ok {
				// The value is stored as a JSON-encoded string: "[\"id1\",\"id2\"]"
				var orderStr string
				if err := json.Unmarshal(orderRaw, &orderStr); err == nil {
					var ids []string
					if err := json.Unmarshal([]byte(orderStr), &ids); err == nil {
						for _, id := range ids {
							id = strings.TrimSpace(id)
							if id != "" && !seen[id] {
								seen[id] = true
								ordered = append(ordered, id)
							}
						}
					}
				}
			}
		}
	}

	// 2. Secondary: scan annotations for any other conversation with pinned:true
	annotGlob := filepath.Join(home, ".gemini", "antigravity", "annotations", "*.pbtxt")
	if files, err := filepath.Glob(annotGlob); err == nil {
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			content := string(b)
			if strings.Contains(content, "pinned:true") || strings.Contains(content, "pinned: true") {
				base := filepath.Base(f)
				cid := strings.TrimSuffix(base, ".pbtxt")
				if cid != "" && !seen[cid] {
					seen[cid] = true
					ordered = append(ordered, cid)
				}
			}
		}
	}

	return ordered, seen
}

// SavePinnedConversations saves the ordered list of pinned IDs to
// ~/.config/Antigravity/app_storage.json and syncs ~/.gemini/antigravity/annotations/*.pbtxt.
func SavePinnedConversations(ordered []string) error {
	pinsMu.Lock()
	defer pinsMu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return fmt.Errorf("user home directory not found")
	}

	// Clean list
	var cleanList []string
	isPinned := make(map[string]bool)
	for _, id := range ordered {
		id = strings.TrimSpace(id)
		if id != "" && !isPinned[id] {
			cleanList = append(cleanList, id)
			isPinned[id] = true
		}
	}

	// 1. Update ~/.config/Antigravity/app_storage.json
	storagePath := filepath.Join(home, ".config", "Antigravity", "app_storage.json")
	var rawMap map[string]any
	if data, err := os.ReadFile(storagePath); err == nil {
		_ = json.Unmarshal(data, &rawMap)
	}
	if rawMap == nil {
		rawMap = make(map[string]any)
	}

	orderJSON, err := json.Marshal(cleanList)
	if err != nil {
		return fmt.Errorf("failed to marshal pinned order: %w", err)
	}
	rawMap["pinned_conversations_order"] = string(orderJSON)

	out, err := json.MarshalIndent(rawMap, "", "  ")
	if err == nil {
		_ = os.WriteFile(storagePath, out, 0644)
	}

	// 2. Sync annotation pbtxt for pinned items
	annotDir := filepath.Join(home, ".gemini", "antigravity", "annotations")
	_ = os.MkdirAll(annotDir, 0755)

	pinnedPattern := regexp.MustCompile(`\s*pinned:\s*(true|false)`)

	// Set pinned:true for pinned IDs
	for _, id := range cleanList {
		pbtxtPath := filepath.Join(annotDir, id+".pbtxt")
		content := ""
		if b, err := os.ReadFile(pbtxtPath); err == nil {
			content = string(b)
		}
		if !strings.Contains(content, "pinned:true") && !strings.Contains(content, "pinned: true") {
			content = pinnedPattern.ReplaceAllString(content, "")
			content = strings.TrimSpace(content)
			if content != "" {
				content += "  pinned:true\n"
			} else {
				content = "pinned:true\n"
			}
			_ = os.WriteFile(pbtxtPath, []byte(content), 0644)
		}
	}

	return nil
}

// TogglePinConversation toggles the pinned state of a conversation and persists it.
// Returns the new pinned state.
func TogglePinConversation(cascadeID string) (bool, error) {
	cascadeID = strings.TrimSpace(cascadeID)
	if cascadeID == "" {
		return false, fmt.Errorf("empty cascade ID")
	}

	ordered, isPinned := LoadPinnedConversations()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false, fmt.Errorf("user home directory not found")
	}

	annotDir := filepath.Join(home, ".gemini", "antigravity", "annotations")
	pbtxtPath := filepath.Join(annotDir, cascadeID+".pbtxt")
	pinnedPattern := regexp.MustCompile(`\s*pinned:\s*(true|false)`)

	if isPinned[cascadeID] {
		// Unpin
		var newOrdered []string
		for _, id := range ordered {
			if id != cascadeID {
				newOrdered = append(newOrdered, id)
			}
		}
		if err := SavePinnedConversations(newOrdered); err != nil {
			return false, err
		}

		// Remove pinned:true from pbtxt
		if b, err := os.ReadFile(pbtxtPath); err == nil {
			cleaned := pinnedPattern.ReplaceAllString(string(b), "")
			cleaned = strings.TrimRight(cleaned, " \t\r\n") + "\n"
			_ = os.WriteFile(pbtxtPath, []byte(cleaned), 0644)
		}
		return false, nil
	}

	// Pin: add to beginning of pinned list
	newOrdered := append([]string{cascadeID}, ordered...)
	if err := SavePinnedConversations(newOrdered); err != nil {
		return false, err
	}
	return true, nil
}
