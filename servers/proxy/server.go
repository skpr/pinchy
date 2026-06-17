// Package proxy implements an HTTP reverse proxy that routes requests of the
// form NAME.pinchy.localhost to the Pod backing the pinchy Environment named
// NAME. A background goroutine polls Kubernetes for Environments and keeps an
// in-memory routing table up to date.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/skpr/pinchy/internal/clientset"
)

// DefaultInterval is how often the watcher re-lists Environments by default.
const DefaultInterval = 5 * time.Second

// ListenParams configures the proxy server.
type ListenParams struct {
	// Ports is the list of ports the proxy listens on. Each listener forwards
	// to the same Pod port on the destination Environment.
	Ports []int
	// Kubeconfig is the path to the kubeconfig file. Empty means in-cluster.
	Kubeconfig string
	// Namespace is the namespace Environments are looked up in.
	Namespace string
	// Interval is how often the watcher re-lists Environments.
	Interval time.Duration
}

// Listen starts the watcher and one HTTP listener per configured port.
// It blocks until one of the servers returns an error or ctx is cancelled.
func Listen(ctx context.Context, params ListenParams) error {
	if len(params.Ports) == 0 {
		return errors.New("at least one port must be configured")
	}

	if params.Interval <= 0 {
		params.Interval = DefaultInterval
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("component", "pinchy-proxy")

	k8sconfig, err := clientcmd.BuildConfigFromFlags("", params.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	pinchyClient, err := clientset.NewForConfig(k8sconfig)
	if err != nil {
		return fmt.Errorf("failed to build pinchy clientset: %w", err)
	}

	table := newRouteTable()

	watcher := &watcher{
		client:    pinchyClient,
		namespace: params.Namespace,
		interval:  params.Interval,
		table:     table,
		logger:    logger,
	}

	group, ctx := errgroup.WithContext(ctx)

	// Run the polling watcher.
	group.Go(func() error {
		return watcher.run(ctx)
	})

	// Run one HTTP server per configured port. The listener port doubles as
	// the destination Pod port.
	for _, port := range params.Ports {
		port := port

		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: newHandler(table, port, logger),
		}

		group.Go(func() error {
			logger.Info("starting listener", "port", port)

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
				return fmt.Errorf("listener on port %d failed: %w", port, err)
			}
		})
	}

	return group.Wait()
}

// routeTable is a concurrency-safe map of Environment name to Pod IP.
type routeTable struct {
	mu     sync.RWMutex
	routes map[string]string
}

func newRouteTable() *routeTable {
	return &routeTable{
		routes: make(map[string]string),
	}
}

// replace atomically swaps the entire routing table.
func (t *routeTable) replace(routes map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes = routes
}

// lookup returns the Pod IP for the given Environment name.
func (t *routeTable) lookup(name string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ip, ok := t.routes[name]
	return ip, ok
}
