package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type streamListener struct {
	dataCh chan json.RawMessage
	endCh  chan error
	onData func(raw json.RawMessage)
}

// Client manages the WebSocket connection and RPC multiplexing to the Antigravity language server hub.
type Client struct {
	hub       *HubInfo
	conn      *websocket.Conn
	writeMu   sync.Mutex
	streamsMu sync.RWMutex
	streams   map[string]*streamListener
	closeCh   chan struct{}
	closeOnce sync.Once
}

// NewClient connects to the Antigravity Hub via WebSocket.
func NewClient(hub *HubInfo) (*Client, error) {
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/connect-websocket", hub.HttpPort)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{fmt.Sprintf("http://127.0.0.1:%d", hub.HttpPort)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial Antigravity hub at %s: %w", wsURL, err)
	}

	c := &Client{
		hub:     hub,
		conn:    conn,
		streams: make(map[string]*streamListener),
		closeCh: make(chan struct{}),
	}

	go c.readLoop()
	go c.heartbeatLoop()

	return c, nil
}

// Close terminates the WebSocket connection and all active listeners.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		if c.conn != nil {
			c.conn.Close()
		}
		c.streamsMu.Lock()
		for id := range c.streams {
			delete(c.streams, id)
		}
		c.streamsMu.Unlock()
	})
	return nil
}

func (c *Client) newStreamID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			c.Close()
			return
		}

		var frame IncomingFrame
		if err := json.Unmarshal(message, &frame); err != nil {
			continue
		}

		c.streamsMu.RLock()
		listener, exists := c.streams[frame.StreamID]
		c.streamsMu.RUnlock()

		if !exists {
			continue
		}

		switch frame.Type {
		case "data":
			if listener.onData != nil {
				listener.onData(frame.Payload)
			}
			select {
			case listener.dataCh <- frame.Payload:
			default:
			}
		case "end":
			var endErr error
			if frame.StatusCode >= 400 {
				endErr = fmt.Errorf("rpc error: status %d (error: %s)", frame.StatusCode, string(frame.Error))
			}
			select {
			case listener.endCh <- endErr:
			default:
			}
			c.streamsMu.Lock()
			delete(c.streams, frame.StreamID)
			c.streamsMu.Unlock()
		}
	}
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			pingFrame := Frame{
				StreamID: c.newStreamID(),
				Type:     "ping",
			}
			data, _ := json.Marshal(pingFrame)
			c.writeMu.Lock()
			_ = c.conn.WriteMessage(websocket.TextMessage, data)
			c.writeMu.Unlock()
		}
	}
}

// UnaryCall executes a single request/response RPC.
func (c *Client) UnaryCall(ctx context.Context, procedure string, req any, resp any) error {
	streamID := c.newStreamID()

	listener := &streamListener{
		dataCh: make(chan json.RawMessage, 1),
		endCh:  make(chan error, 1),
	}

	c.streamsMu.Lock()
	c.streams[streamID] = listener
	c.streamsMu.Unlock()

	defer func() {
		c.streamsMu.Lock()
		delete(c.streams, streamID)
		c.streamsMu.Unlock()
	}()

	headers := map[string][]string{
		"Content-Type":         {"application/json"},
		"x-codeium-csrf-token": {c.hub.CsrfToken},
	}

	frame := Frame{
		StreamID:  streamID,
		Type:      "start",
		Procedure: procedure,
		Stream:    false,
		Headers:   headers,
		Payload:   req,
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	c.writeMu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	var rawResp json.RawMessage
	select {
	case <-ctx.Done():
		c.cancelStream(streamID)
		return ctx.Err()
	case <-c.closeCh:
		return fmt.Errorf("connection closed")
	case raw := <-listener.dataCh:
		rawResp = raw
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-listener.endCh:
		if err != nil {
			return err
		}
	case <-time.After(3 * time.Second):
		// Unary call might not send 'end' or end could arrive slightly later
	}

	if resp != nil && len(rawResp) > 0 {
		if err := json.Unmarshal(rawResp, resp); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// StartStream begins a server streaming RPC.
func (c *Client) StartStream(ctx context.Context, procedure string, req any, onData func(raw json.RawMessage)) (func(), error) {
	streamID := c.newStreamID()

	listener := &streamListener{
		dataCh: make(chan json.RawMessage, 50),
		endCh:  make(chan error, 1),
		onData: onData,
	}

	c.streamsMu.Lock()
	c.streams[streamID] = listener
	c.streamsMu.Unlock()

	headers := map[string][]string{
		"Content-Type":         {"application/connect+json"},
		"x-codeium-csrf-token": {c.hub.CsrfToken},
	}

	frame := Frame{
		StreamID:  streamID,
		Type:      "start",
		Procedure: procedure,
		Stream:    true,
		Headers:   headers,
		Payload:   req,
	}

	data, err := json.Marshal(frame)
	if err != nil {
		c.streamsMu.Lock()
		delete(c.streams, streamID)
		c.streamsMu.Unlock()
		return nil, fmt.Errorf("failed to marshal stream frame: %w", err)
	}

	c.writeMu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	if err != nil {
		c.streamsMu.Lock()
		delete(c.streams, streamID)
		c.streamsMu.Unlock()
		return nil, fmt.Errorf("failed to write stream frame: %w", err)
	}

	cancelFunc := func() {
		c.cancelStream(streamID)
	}

	return cancelFunc, nil
}

func (c *Client) cancelStream(streamID string) {
	cancelFrame := Frame{
		StreamID: streamID,
		Type:     "cancel",
	}
	data, _ := json.Marshal(cancelFrame)
	c.writeMu.Lock()
	_ = c.conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()

	c.streamsMu.Lock()
	delete(c.streams, streamID)
	c.streamsMu.Unlock()
}

// ConversationItem represents a unified conversation for display in the UI.
type ConversationItem struct {
	CascadeID        string
	Title            string
	Snippet          string
	WorkspaceName    string
	StepCount        int
	Status           string
	LastModifiedTime string
}

// SearchConversations queries the conversation index across all workspaces.
func (c *Client) SearchConversations(ctx context.Context, query string, limit int) ([]ConversationSearchResult, error) {
	if limit <= 0 {
		limit = 100
	}
	req := map[string]any{
		"query": query,
		"limit": limit,
	}
	var resp SearchConversationsPayload
	err := c.UnaryCall(
		ctx,
		"/exa.language_server_pb.LanguageServerService/SearchConversations",
		req,
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// ListAllConversations returns all historical and active conversations, merged and sorted newest first.
func (c *Client) ListAllConversations(ctx context.Context) ([]ConversationItem, error) {
	// 1. Fetch from search index
	searchResults, err := c.SearchConversations(ctx, "", 150)
	if err != nil {
		// Fallback to active trajectories if search fails
		searchResults = nil
	}

	// 2. Fetch active in-memory trajectories
	activeTrajs, _ := c.GetAllTrajectories(ctx)

	itemMap := make(map[string]*ConversationItem)

	for _, r := range searchResults {
		title := r.Title
		if title == "" {
			title = r.CascadeID
		}
		itemMap[r.CascadeID] = &ConversationItem{
			CascadeID:        r.CascadeID,
			Title:            title,
			Snippet:          r.Snippet,
			WorkspaceName:    r.WorkspaceName,
			Status:           "IDLE",
			LastModifiedTime: r.LastModifiedTime,
		}
	}

	for id, t := range activeTrajs {
		item, exists := itemMap[id]
		if !exists {
			title := t.Summary
			if t.Annotations != nil && t.Annotations.Title != "" {
				title = t.Annotations.Title
			}
			if title == "" {
				title = id
			}
			item = &ConversationItem{
				CascadeID:        id,
				Title:            title,
				LastModifiedTime: t.LastModifiedTime,
			}
			itemMap[id] = item
		}
		item.StepCount = t.StepCount
		if t.Status != "" {
			item.Status = t.Status
		}
		if t.LastModifiedTime > item.LastModifiedTime {
			item.LastModifiedTime = t.LastModifiedTime
		}
	}

	var list []ConversationItem
	for _, it := range itemMap {
		list = append(list, *it)
	}

	// Sort newest first
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastModifiedTime > list[j].LastModifiedTime
	})

	return list, nil
}

// GetAllTrajectories returns all conversation trajectories tracked by the language server.
func (c *Client) GetAllTrajectories(ctx context.Context) (map[string]TrajectorySummary, error) {
	var resp GetAllTrajectoriesPayload
	err := c.UnaryCall(
		ctx,
		"/exa.language_server_pb.LanguageServerService/GetAllCascadeTrajectories",
		map[string]any{},
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return resp.TrajectorySummaries, nil
}

// GetTrajectorySteps fetches full message history, thoughts, and tool executions for a conversation.
func (c *Client) GetTrajectorySteps(ctx context.Context, cascadeID string) ([]Step, error) {
	var resp GetCascadeTrajectoryStepsPayload
	err := c.UnaryCall(
		ctx,
		"/exa.language_server_pb.LanguageServerService/GetCascadeTrajectorySteps",
		map[string]string{"cascadeId": cascadeID},
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return resp.Steps, nil
}

// SendMessage sends a user prompt message to the given conversation.
func (c *Client) SendMessage(ctx context.Context, cascadeID, text string) error {
	req := map[string]any{
		"cascadeId": cascadeID,
		"items": []map[string]string{
			{"text": text},
		},
		"metadata": map[string]any{},
	}
	return c.UnaryCall(
		ctx,
		"/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage",
		req,
		nil,
	)
}

// CancelCascade stops or cancels the current turn in a conversation.
func (c *Client) CancelCascade(ctx context.Context, cascadeID string) error {
	return c.UnaryCall(
		ctx,
		"/exa.language_server_pb.LanguageServerService/CancelCascadeInvocation",
		map[string]string{"cascadeId": cascadeID},
		nil,
	)
}

// StreamAgentStateUpdates subscribes to live state updates (tokens, thoughts, step completions).
func (c *Client) StreamAgentStateUpdates(ctx context.Context, cascadeID, subscriberID string, onData func(payload json.RawMessage)) (func(), error) {
	req := map[string]string{
		"conversationId": cascadeID,
		"subscriberId":   subscriberID,
	}
	return c.StartStream(
		ctx,
		"/exa.language_server_pb.LanguageServerService/StreamAgentStateUpdates",
		req,
		onData,
	)
}
