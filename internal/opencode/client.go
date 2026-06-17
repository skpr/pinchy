// Package opencode provides a small HTTP client for the opencode server API
// (https://opencode.ai/docs/server). It exposes only the endpoints the pinchy
// kanban board needs: listing projects, sessions and their statuses, and the
// interactive actions (abort, delete, permission responses).
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the opencode server address used when none is configured.
const DefaultBaseURL = "http://localhost:4096"

// Client talks to a single opencode server over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
}

// Option configures a Client.
type Option func(*Client)

// WithBasicAuth sets HTTP basic auth credentials, matching opencode's
// OPENCODE_SERVER_USERNAME / OPENCODE_SERVER_PASSWORD support.
func WithBasicAuth(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithHTTPClient overrides the underlying http.Client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		c.httpClient = h
	}
}

// New returns a Client for the opencode server at baseURL. An empty baseURL
// falls back to DefaultBaseURL.
func New(baseURL string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// BaseURL returns the configured opencode server base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Project mirrors the opencode Project type. A project is what pinchy presents
// as a "workspace" swimlane on the board.
type Project struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
	VCSDir   string `json:"vcsDir,omitempty"`
	VCS      string `json:"vcs,omitempty"`
	Time     struct {
		Created     int64 `json:"created"`
		Initialized int64 `json:"initialized,omitempty"`
	} `json:"time"`
}

// SessionSummary holds the diff totals opencode tracks for a session.
type SessionSummary struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Files     int `json:"files"`
}

// Session mirrors the subset of the opencode Session type the board renders.
type Session struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"projectID"`
	Directory string          `json:"directory"`
	ParentID  string          `json:"parentID,omitempty"`
	Title     string          `json:"title"`
	Summary   *SessionSummary `json:"summary,omitempty"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// SessionStatus mirrors the opencode SessionStatus union. Only Type is always
// present; the remaining fields are populated for the "retry" variant.
type SessionStatus struct {
	Type    string `json:"type"`
	Attempt int    `json:"attempt,omitempty"`
	Message string `json:"message,omitempty"`
	Next    int64  `json:"next,omitempty"`
}

// messageEnvelope is the shape of an item returned by /session/:id/message.
// We only care whether the latest assistant message carries an error.
type messageEnvelope struct {
	Info struct {
		Role  string          `json:"role"`
		Error json.RawMessage `json:"error,omitempty"`
	} `json:"info"`
}

// ListProjects returns all projects known to the opencode server.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	if err := c.do(ctx, http.MethodGet, "/project", nil, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// ListSessions returns all sessions known to the opencode server.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	var sessions []Session
	if err := c.do(ctx, http.MethodGet, "/session", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// SessionStatuses returns the current status for every session, keyed by
// session ID.
func (c *Client) SessionStatuses(ctx context.Context) (map[string]SessionStatus, error) {
	statuses := map[string]SessionStatus{}
	if err := c.do(ctx, http.MethodGet, "/session/status", nil, &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

// LatestMessageHasError reports whether the most recent message in the session
// is an errored assistant message. It returns false (no error) when the session
// has no messages.
func (c *Client) LatestMessageHasError(ctx context.Context, sessionID string) (bool, error) {
	path := fmt.Sprintf("/session/%s/message?limit=1", url.PathEscape(sessionID))

	var messages []messageEnvelope
	if err := c.do(ctx, http.MethodGet, path, nil, &messages); err != nil {
		return false, err
	}

	if len(messages) == 0 {
		return false, nil
	}

	last := messages[len(messages)-1]
	return len(last.Info.Error) > 0 && string(last.Info.Error) != "null", nil
}

// AbortSession aborts a running session.
func (c *Client) AbortSession(ctx context.Context, sessionID string) error {
	path := fmt.Sprintf("/session/%s/abort", url.PathEscape(sessionID))
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DeleteSession deletes a session and all of its data.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	path := fmt.Sprintf("/session/%s", url.PathEscape(sessionID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// RespondPermission replies to a pending permission request. response is
// typically "once", "always" or "reject" depending on the opencode build; the
// board forwards whatever the frontend supplies.
func (c *Client) RespondPermission(ctx context.Context, sessionID, permissionID, response string) error {
	path := fmt.Sprintf("/session/%s/permissions/%s",
		url.PathEscape(sessionID), url.PathEscape(permissionID))

	body := map[string]string{"response": response}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

// do performs an HTTP request against the opencode server. When body is
// non-nil it is JSON-encoded as the request body. When out is non-nil the
// response body is JSON-decoded into it.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.password != "" {
		username := c.username
		if username == "" {
			username = "opencode"
		}
		req.SetBasicAuth(username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("opencode returned %d for %s %s: %s",
			resp.StatusCode, method, path, strings.TrimSpace(string(snippet)))
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response from %s %s: %w", method, path, err)
	}

	return nil
}
