package board

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/skpr/pinchy/internal/opencode"
	"github.com/skpr/pinchy/proto/pb"
)

//go:embed web
var webFS embed.FS

// envConfig holds the environment-link settings passed to the board builder.
type envConfig struct {
	ports  []int
	domain string
	scheme string
}

// newHandler builds the board's HTTP handler: the JSON board API, the card
// action endpoints, the runtime config, and the embedded single-page app.
func newHandler(client *opencode.Client, tracker *opencode.PermissionTracker, webURL string, logger *slog.Logger, env envConfig, envClient pb.EnvironmentClient) (http.Handler, error) {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}

	b := &builder{
		client:      client,
		permissions: tracker,
		logger:      logger,
		envPorts:    env.ports,
		envDomain:   env.domain,
		envScheme:   env.scheme,
	}

	mux := http.NewServeMux()

	// Board snapshot, polled by the frontend.
	mux.HandleFunc("GET /api/board", func(w http.ResponseWriter, r *http.Request) {
		board, err := b.build(r.Context())
		if err != nil {
			logger.Error("failed to build board", "error", err)
			writeError(w, http.StatusBadGateway, "failed to reach opencode server")
			return
		}
		writeJSON(w, http.StatusOK, board)
	})

	// Runtime config for the frontend (e.g. where opencode web lives).
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"opencodeWebURL": strings.TrimRight(webURL, "/")})
	})

	// Abort a running session.
	mux.HandleFunc("POST /api/sessions/{id}/abort", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := client.AbortSession(r.Context(), id); err != nil {
			logger.Error("failed to abort session", "session", id, "error", err)
			writeError(w, http.StatusBadGateway, "failed to abort session")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Delete a session.
	mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := client.DeleteSession(r.Context(), id); err != nil {
			logger.Error("failed to delete session", "session", id, "error", err)
			writeError(w, http.StatusBadGateway, "failed to delete session")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Respond to a pending permission prompt.
	mux.HandleFunc("POST /api/sessions/{id}/permission", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var body struct {
			PermissionID string `json:"permissionID"`
			Response     string `json:"response"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.PermissionID == "" || body.Response == "" {
			writeError(w, http.StatusBadRequest, "permissionID and response are required")
			return
		}

		if err := client.RespondPermission(r.Context(), id, body.PermissionID, body.Response); err != nil {
			logger.Error("failed to respond to permission", "session", id, "error", err)
			writeError(w, http.StatusBadGateway, "failed to respond to permission")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// List environments via the pinchy-api gRPC server.
	mux.HandleFunc("GET /api/environments", func(w http.ResponseWriter, r *http.Request) {
		resp, err := envClient.List(r.Context(), &pb.ListRequest{})
		if err != nil {
			logger.Error("failed to list environments", "error", err)
			writeError(w, http.StatusBadGateway, "failed to reach pinchy-api")
			return
		}
		writeJSON(w, http.StatusOK, resp.GetEnvironments())
	})

	// Delete an environment by name.
	mux.HandleFunc("DELETE /api/environments/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		_, err := envClient.Delete(r.Context(), &pb.DeleteRequest{Name: name})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				writeError(w, http.StatusNotFound, "environment not found")
				return
			}
			logger.Error("failed to delete environment", "name", name, "error", err)
			writeError(w, http.StatusBadGateway, "failed to delete environment")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Embedded single-page app.
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	return mux, nil
}

// writeJSON serialises v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
