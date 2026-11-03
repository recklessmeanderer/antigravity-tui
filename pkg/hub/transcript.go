package hub

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type rawTranscriptLine struct {
	StepIndex int               `json:"step_index"`
	Source    string            `json:"source"`
	Type      string            `json:"type"`
	Status    string            `json:"status"`
	CreatedAt string            `json:"created_at"`
	Content   string            `json:"content"`
	Thinking  string            `json:"thinking"`
	ToolCalls []rawToolCallLine `json:"tool_calls"`
}

type rawToolCallLine struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

var userReqRegex = regexp.MustCompile(`(?s)<USER_REQUEST>(.*?)</USER_REQUEST>`)

// LoadTranscriptFromDisk loads the latest full conversation transcript directly from
// ~/.gemini/antigravity/brain/<cascadeID>/.system_generated/logs/transcript.jsonl.
// This captures all recent turns in real time even when the language server memory cache lags.
func LoadTranscriptFromDisk(cascadeID string) ([]Step, error) {
	if cascadeID == "" {
		return nil, os.ErrNotExist
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".gemini/antigravity/brain", cascadeID, ".system_generated/logs/transcript.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var steps []Step
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw rawTranscriptLine
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		switch raw.Type {
		case "USER_INPUT":
			text := raw.Content
			if m := userReqRegex.FindStringSubmatch(text); len(m) > 1 {
				text = strings.TrimSpace(m[1])
			}
			steps = append(steps, Step{
				Type:   "CORTEX_STEP_TYPE_USER_INPUT",
				Status: "CORTEX_STEP_STATUS_DONE",
				UserInput: &UserInput{
					UserResponse: text,
					Items: []UserInputItem{
						{Text: text},
					},
				},
			})

		case "PLANNER_RESPONSE":
			var toolCalls []ToolCall
			for _, tc := range raw.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					Name:          tc.Name,
					ArgumentsJSON: string(tc.Args),
				})
			}

			steps = append(steps, Step{
				Type:   "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				Status: "CORTEX_STEP_STATUS_DONE",
				PlannerResponse: &PlannerResponse{
					Thinking:  raw.Thinking,
					Response:  raw.Content,
					ToolCalls: toolCalls,
				},
			})

		case "GENERIC":
			if strings.Contains(raw.Content, "Command output:") || strings.Contains(raw.Content, "Command exited with code") {
				rawJSON, _ := json.Marshal(raw.Content)
				steps = append(steps, Step{
					Type:   "CORTEX_STEP_TYPE_RUN_COMMAND",
					Status: "CORTEX_STEP_STATUS_DONE",
					RunCommand: &RunCommandStep{
						CombinedOutput: rawJSON,
					},
				})
			}
		}
	}

	if len(steps) == 0 {
		return nil, os.ErrNotExist
	}

	return steps, nil
}
