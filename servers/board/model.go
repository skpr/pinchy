package board

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/skpr/pinchy/internal/opencode"
)

// Status is a derived kanban column. Opencode only reports idle/busy/retry, so
// the board combines that with pending permissions and message errors to
// produce a more actionable set of columns.
type Status string

const (
	// StatusWorking is a session opencode is actively processing.
	StatusWorking Status = "working"
	// StatusNeedsInput is a session waiting on a permission response.
	StatusNeedsInput Status = "needs-input"
	// StatusIdle is a session with nothing in progress.
	StatusIdle Status = "idle"
	// StatusError is a session whose latest message ended in an error.
	StatusError Status = "error"
)

// Columns is the ordered list of columns rendered for each swimlane.
var Columns = []Status{StatusWorking, StatusNeedsInput, StatusIdle, StatusError}

// EnvLink is a link to a session's environment, served by pinchy-proxy on a
// particular port.
type EnvLink struct {
	Port int    `json:"port"`
	URL  string `json:"url"`
}

// Card is a single session as rendered on the board.
type Card struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Status       Status    `json:"status"`
	Updated      int64     `json:"updated"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	PermissionID string    `json:"permissionID,omitempty"`
	EnvName      string    `json:"envName,omitempty"`
	Envs         []EnvLink `json:"envs,omitempty"`
}

// Swimlane groups the sessions for a single working directory. Opencode reports
// every session under one "global" project when run from a non-git root, so the
// board groups by the session's directory instead, which is what distinguishes
// the user's workspaces.
type Swimlane struct {
	Directory string  `json:"directory"`
	Cards     []*Card `json:"cards"`
}

// Board is the full payload returned to the frontend.
type Board struct {
	Columns   []Status    `json:"columns"`
	Swimlanes []*Swimlane `json:"swimlanes"`
}

// builder assembles a Board from the opencode server. It is created per request
// so it carries no mutable state between polls.
type builder struct {
	client      *opencode.Client
	permissions *opencode.PermissionTracker
	logger      *slog.Logger

	// Environment link settings, used to build the pinchy-proxy URLs for each
	// session's environment.
	envPorts  []int
	envDomain string
	envScheme string
}

// build fetches sessions and statuses and assembles the board, grouping
// sessions into one swimlane per working directory.
func (b *builder) build(ctx context.Context) (*Board, error) {
	sessions, err := b.client.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	statuses, err := b.client.SessionStatuses(ctx)
	if err != nil {
		return nil, err
	}

	// Group sessions into swimlanes keyed by their directory.
	lanesByDir := make(map[string]*Swimlane)

	for i := range sessions {
		session := &sessions[i]

		// Child sessions (subagents) are part of their parent's work and would
		// clutter the board, so they are not shown as top-level cards.
		if session.ParentID != "" {
			continue
		}

		lane, ok := lanesByDir[session.Directory]
		if !ok {
			lane = &Swimlane{Directory: session.Directory, Cards: []*Card{}}
			lanesByDir[session.Directory] = lane
		}

		lane.Cards = append(lane.Cards, b.cardFor(ctx, session, statuses))
	}

	// Order lanes alphabetically by directory for a stable layout, and sort
	// cards within each lane by most recently updated.
	lanes := make([]*Swimlane, 0, len(lanesByDir))
	for _, lane := range lanesByDir {
		sort.SliceStable(lane.Cards, func(i, j int) bool {
			return lane.Cards[i].Updated > lane.Cards[j].Updated
		})
		lanes = append(lanes, lane)
	}
	sort.SliceStable(lanes, func(i, j int) bool {
		return lanes[i].Directory < lanes[j].Directory
	})

	return &Board{Columns: Columns, Swimlanes: lanes}, nil
}

// cardFor builds the card for a single session, deriving its status.
func (b *builder) cardFor(ctx context.Context, session *opencode.Session, statuses map[string]opencode.SessionStatus) *Card {
	card := &Card{
		ID:      session.ID,
		Title:   session.Title,
		Updated: session.Time.Updated,
	}
	if session.Summary != nil {
		card.Additions = session.Summary.Additions
		card.Deletions = session.Summary.Deletions
	}

	status := statuses[session.ID]
	pending, hasPending := b.permissions.Pending(session.ID)

	card.Status = b.deriveStatus(ctx, session.ID, status, hasPending)
	if card.Status == StatusNeedsInput {
		card.PermissionID = pending.PermissionID
	}

	// Build links to the session's environment, served by pinchy-proxy. The
	// environment is named after the sanitized session ID, matching how the
	// pinchy API server names the Environment resource.
	card.EnvName = rfc1123Subdomain(session.ID)
	for _, port := range b.envPorts {
		card.Envs = append(card.Envs, EnvLink{
			Port: port,
			URL:  fmt.Sprintf("%s://%s.%s:%d", b.envScheme, card.EnvName, b.envDomain, port),
		})
	}

	return card
}

// deriveStatus maps an opencode SessionStatus plus side signals onto a board
// column. Precedence is: needs-input > working > error > idle. A pending
// permission is the most actionable state and is checked first. Errors are only
// inspected for otherwise-idle sessions to avoid an extra API call per busy
// card on every poll.
func (b *builder) deriveStatus(ctx context.Context, sessionID string, status opencode.SessionStatus, hasPending bool) Status {
	if hasPending {
		return StatusNeedsInput
	}

	switch status.Type {
	case "busy", "retry":
		return StatusWorking
	}

	// Idle session: check whether its last message errored.
	errored, err := b.client.LatestMessageHasError(ctx, sessionID)
	if err != nil {
		b.logger.Debug("failed to check session error state", "session", sessionID, "error", err)
		return StatusIdle
	}
	if errored {
		return StatusError
	}

	return StatusIdle
}

// matches any run of characters that are NOT lowercase alphanumeric, '-' or '.'
var invalidChars = regexp.MustCompile(`[^a-z0-9.-]+`)

// matches leading/trailing characters that aren't alphanumeric
var trimEdges = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)

// rfc1123Subdomain converts s into an RFC 1123 subdomain, matching the
// sanitization the pinchy API server applies when naming Environment resources
// (servers/api/environment.RFC1123Subdomain). It is duplicated here so the
// board binary does not depend on the gRPC/Kubernetes-heavy api package.
func rfc1123Subdomain(s string) string {
	s = strings.ToLower(s)
	s = invalidChars.ReplaceAllString(s, "-")
	s = trimEdges.ReplaceAllString(s, "")

	if len(s) > 253 {
		s = s[:253]
		s = trimEdges.ReplaceAllString(s, "")
	}
	return s
}
