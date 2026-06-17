// Package board implements an HTTP server that renders a kanban board of
// opencode sessions across all projects. It reads session data from a single
// opencode server's HTTP API, derives a status column for each session, and
// serves an embedded single-page app that polls for updates and drives the
// interactive card actions (abort, delete, permission responses).
package board

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/skpr/pinchy/internal/opencode"
	"github.com/skpr/pinchy/proto/pb"
)

// ListenParams configures the board server.
type ListenParams struct {
	// Port is the port the board HTTP server listens on.
	Port int
	// OpencodeURL is the base URL of the opencode server to read sessions from.
	OpencodeURL string
	// OpencodeWebURL is the base URL the frontend links to when opening a
	// session in the opencode web UI. Defaults to OpencodeURL when empty.
	OpencodeWebURL string
	// OpencodePassword, when set, enables HTTP basic auth against the opencode
	// server (matching OPENCODE_SERVER_PASSWORD).
	OpencodePassword string
	// EnvPorts are the pinchy-proxy ports each session's environment is linked
	// on. Defaults to 8080 and 3000 when empty.
	EnvPorts []int
	// EnvDomain is the host suffix pinchy-proxy serves environments under.
	// Defaults to "pinchy.localhost" when empty.
	EnvDomain string
	// EnvScheme is the URL scheme for environment links. Defaults to "http".
	EnvScheme string
	// APIServer is the address of the pinchy-api gRPC server used to list and
	// delete environments. Defaults to "localhost:50051".
	APIServer string
}

// Environment link defaults, matching pinchy-proxy's defaults.
const (
	defaultEnvDomain = "pinchy.localhost"
	defaultEnvScheme = "http"
)

var defaultEnvPorts = []int{8080, 3000}

const defaultAPIServer = "localhost:50051"

// Listen starts the board HTTP server and the background permission tracker.
// It blocks until one of them returns an error or ctx is cancelled.
func Listen(ctx context.Context, params ListenParams) error {
	if params.Port == 0 {
		return errors.New("a port must be configured")
	}

	if len(params.EnvPorts) == 0 {
		params.EnvPorts = defaultEnvPorts
	}
	if params.EnvDomain == "" {
		params.EnvDomain = defaultEnvDomain
	}
	if params.EnvScheme == "" {
		params.EnvScheme = defaultEnvScheme
	}
	if params.APIServer == "" {
		params.APIServer = defaultAPIServer
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("component", "pinchy-board")

	var opts []opencode.Option
	if params.OpencodePassword != "" {
		opts = append(opts, opencode.WithBasicAuth("opencode", params.OpencodePassword))
	}
	client := opencode.New(params.OpencodeURL, opts...)

	tracker := opencode.NewPermissionTracker(client, logger)

	webURL := params.OpencodeWebURL
	if webURL == "" {
		webURL = client.BaseURL()
	}

	// Dial the pinchy-api gRPC server. grpc.NewClient is non-blocking; the
	// connection is established lazily so the board starts even if the API is
	// temporarily down.
	conn, err := grpc.NewClient(params.APIServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create grpc client for %s: %w", params.APIServer, err)
	}
	envClient := pb.NewEnvironmentClient(conn)

	handler, err := newHandler(client, tracker, webURL, logger, envConfig{
		ports:  params.EnvPorts,
		domain: params.EnvDomain,
		scheme: params.EnvScheme,
	}, envClient)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to build handler: %w", err)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", params.Port),
		Handler: handler,
	}

	group, ctx := errgroup.WithContext(ctx)

	defer conn.Close()

	// Background SSE consumer that keeps the pending-permission state current.
	group.Go(func() error {
		return tracker.Run(ctx)
	})

	// The board HTTP server.
	group.Go(func() error {
		logger.Info("starting board server", "port", params.Port, "opencode", client.BaseURL())

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.ListenAndServe()
		}()

		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
			return ctx.Err()
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("board server failed: %w", err)
		}
	})

	return group.Wait()
}
