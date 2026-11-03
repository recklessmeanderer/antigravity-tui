package hub

import "encoding/json"

// Frame is the WebSocket RPC message envelope.
type Frame struct {
	StreamID   string              `json:"streamId"`
	Type       string              `json:"type"` // "start", "data", "end", "cancel", "ping"
	Procedure  string              `json:"procedure,omitempty"`
	Stream     bool                `json:"stream,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Payload    any                 `json:"payload,omitempty"`
	StatusCode int                 `json:"statusCode,omitempty"`
	Error      any                 `json:"error,omitempty"`
}

// IncomingFrame represents a raw frame received from the server.
type IncomingFrame struct {
	StreamID   string          `json:"streamId"`
	Type       string          `json:"type"` // "data", "end", "ping"
	Payload    json.RawMessage `json:"payload,omitempty"`
	StatusCode int             `json:"statusCode,omitempty"`
	Error      json.RawMessage `json:"error,omitempty"`
}

// TrajectorySummary is the metadata for a conversation.
type TrajectorySummary struct {
	Summary          string                 `json:"summary"`
	StepCount        int                    `json:"stepCount"`
	LastModifiedTime string                 `json:"lastModifiedTime"`
	TrajectoryID     string                 `json:"trajectoryId"`
	Status           string                 `json:"status"`
	CreatedTime      string                 `json:"createdTime"`
	Annotations      *TrajectoryAnnotations `json:"annotations,omitempty"`
}

type TrajectoryAnnotations struct {
	Title string `json:"title,omitempty"`
}

type GetAllTrajectoriesPayload struct {
	TrajectorySummaries map[string]TrajectorySummary `json:"trajectorySummaries"`
}

type ConversationSearchResult struct {
	CascadeID        string `json:"cascadeId"`
	Title            string `json:"title"`
	WorkspaceName    string `json:"workspaceName"`
	Snippet          string `json:"snippet"`
	LastModifiedTime string `json:"lastModifiedTime"`
	NeedsAttention   bool   `json:"needsAttention"`
}

type SearchConversationsPayload struct {
	Results []ConversationSearchResult `json:"results"`
}

// Step represents a step in an Antigravity conversation trajectory.
type Step struct {
	Type            string           `json:"type"`
	Status          string           `json:"status"`
	UserInput       *UserInput       `json:"userInput,omitempty"`
	PlannerResponse *PlannerResponse `json:"plannerResponse,omitempty"`
	RunCommand      *RunCommandStep  `json:"runCommand,omitempty"`
	Generic         *GenericStep     `json:"generic,omitempty"`
	ViewFile        *ViewFileStep    `json:"viewFile,omitempty"`
	SearchWeb       *SearchWebStep   `json:"searchWeb,omitempty"`
	McpTool         *McpToolStep     `json:"mcpTool,omitempty"`
	CodeAction      *CodeActionStep  `json:"codeAction,omitempty"`
}

type UserInput struct {
	UserResponse string          `json:"userResponse"`
	Items        []UserInputItem `json:"items"`
}

type UserInputItem struct {
	Text string `json:"text"`
}

type PlannerResponse struct {
	Thinking  string     `json:"thinking"`
	Response  string     `json:"response"`
	MessageID string     `json:"messageId"`
	ToolCalls []ToolCall `json:"toolCalls"`
}

type ToolCall struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArgumentsJSON string `json:"argumentsJson"`
}

type RunCommandStep struct {
	CommandLine    string          `json:"commandLine"`
	Output         string          `json:"output"`
	CombinedOutput json.RawMessage `json:"combinedOutput,omitempty"`
	ExitCode       int             `json:"exitCode"`
}

func (s *RunCommandStep) GetOutput() string {
	if len(s.CombinedOutput) > 0 {
		var str string
		if err := json.Unmarshal(s.CombinedOutput, &str); err == nil && str != "" {
			return str
		}
		var obj struct {
			Full string `json:"full"`
		}
		if err := json.Unmarshal(s.CombinedOutput, &obj); err == nil && obj.Full != "" {
			return obj.Full
		}
	}
	return s.Output
}

type GenericStep struct {
	Content string `json:"content"`
}

type ViewFileStep struct {
	AbsolutePathURI string `json:"absolutePathUri"`
	Content         string `json:"content"`
}

type SearchWebStep struct {
	Query   string `json:"query"`
	Summary string `json:"summary"`
}

type McpToolStep struct {
	ServerName   string   `json:"serverName"`
	ToolCall     ToolCall `json:"toolCall"`
	ResultString string   `json:"resultString"`
}

type CodeActionStep struct {
	ActionSpec struct {
		CreateFile *struct {
			Path struct {
				AbsoluteURI string `json:"absoluteUri"`
			} `json:"path"`
		} `json:"createFile"`
	} `json:"actionSpec"`
}

type GetCascadeTrajectoryStepsPayload struct {
	Steps []Step `json:"steps"`
}

type StreamAgentStateUpdate struct {
	Update *AgentStateUpdate `json:"update,omitempty"`
}

type AgentStateUpdate struct {
	ConversationID   string          `json:"conversationId"`
	TrajectoryID     string          `json:"trajectoryId"`
	Status           string          `json:"status"`
	ExecutableStatus string          `json:"executableStatus"`
	RawPayload       json.RawMessage `json:"-"`
}
