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

// Step represents a step in an Antigravity conversation trajectory.
type Step struct {
	Type            string           `json:"type"`
	Status          string           `json:"status"`
	UserInput       *UserInput       `json:"userInput,omitempty"`
	PlannerResponse *PlannerResponse `json:"plannerResponse,omitempty"`
	RunCommand      *RunCommandStep  `json:"runCommand,omitempty"`
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
	MessageID string     `json:"messageId"`
	ToolCalls []ToolCall `json:"toolCalls"`
}

type ToolCall struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArgumentsJSON string `json:"argumentsJson"`
}

type RunCommandStep struct {
	CommandLine string `json:"commandLine"`
	Output      string `json:"output"`
	ExitCode    int    `json:"exitCode"`
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
