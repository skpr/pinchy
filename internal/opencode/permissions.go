package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PendingPermission identifies a permission request awaiting a response on a
// particular session.
type PendingPermission struct {
	PermissionID string
	Title        string
}

// PermissionTracker maintains the set of pending permission requests per
// session by consuming the opencode server's /event SSE stream. The board uses
// this to surface "Needs input" sessions and to respond to prompts, since the
// opencode API has no endpoint to list outstanding permissions directly.
//
// All exported methods are safe for concurrent use.
type PermissionTracker struct {
	client *Client
	logger *slog.Logger

	mu      sync.RWMutex
	pending map[string]PendingPermission // sessionID -> pending permission
}

// NewPermissionTracker returns a tracker that reads events from the given
// client's opencode server.
func NewPermissionTracker(client *Client, logger *slog.Logger) *PermissionTracker {
	return &PermissionTracker{
		client:  client,
		logger:  logger,
		pending: make(map[string]PendingPermission),
	}
}

// Pending returns the pending permission for a session, if any.
func (t *PermissionTracker) Pending(sessionID string) (PendingPermission, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	p, ok := t.pending[sessionID]
	return p, ok
}

// Run consumes the SSE stream until ctx is cancelled, reconnecting on failure.
// A dropped connection or transient error must never be fatal, so errors are
// logged and retried after a short backoff.
func (t *PermissionTracker) Run(ctx context.Context) error {
	const backoff = 2 * time.Second

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := t.stream(ctx); err != nil && ctx.Err() == nil {
			t.logger.Warn("event stream disconnected, retrying", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// sseEvent is the decoded payload of a single opencode bus event.
type sseEvent struct {
	Type       string `json:"type"`
	Properties struct {
		// permission.updated carries the full Permission object.
		ID        string `json:"id"`
		SessionID string `json:"sessionID"`
		Title     string `json:"title"`
		// permission.replied carries these.
		PermissionID string `json:"permissionID"`
	} `json:"properties"`
}

// stream opens a single SSE connection and processes events until it ends.
func (t *PermissionTracker) stream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.client.baseURL+"/event", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if t.client.password != "" {
		username := t.client.username
		if username == "" {
			username = "opencode"
		}
		req.SetBasicAuth(username, t.client.password)
	}

	// The SSE connection is long-lived, so it must not inherit the client's
	// short request timeout.
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}

		data = strings.TrimSpace(data)
		if data == "" {
			continue
		}

		var event sseEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		t.handle(event)
	}

	return scanner.Err()
}

// handle updates the pending-permission map based on a single event.
func (t *PermissionTracker) handle(event sseEvent) {
	switch event.Type {
	case "permission.updated":
		if event.Properties.SessionID == "" || event.Properties.ID == "" {
			return
		}
		t.mu.Lock()
		t.pending[event.Properties.SessionID] = PendingPermission{
			PermissionID: event.Properties.ID,
			Title:        event.Properties.Title,
		}
		t.mu.Unlock()

	case "permission.replied":
		if event.Properties.SessionID == "" {
			return
		}
		t.mu.Lock()
		delete(t.pending, event.Properties.SessionID)
		t.mu.Unlock()

	case "session.deleted", "session.idle":
		// A session that went idle or was deleted has no outstanding prompt.
		if event.Properties.SessionID != "" {
			t.mu.Lock()
			delete(t.pending, event.Properties.SessionID)
			t.mu.Unlock()
		}
	}
}
