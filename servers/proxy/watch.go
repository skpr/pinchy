package proxy

import (
	"context"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skpr/pinchy/apis/pinchy/v1beta1"
	"github.com/skpr/pinchy/internal/clientset"
)

// watcher periodically polls the Kubernetes API for Environments and rebuilds
// the routing table from their reported Pod IPs.
type watcher struct {
	client    *clientset.Clientset
	namespace string
	interval  time.Duration
	table     *routeTable
	logger    *slog.Logger
}

// run polls until the context is cancelled. It refreshes once immediately so
// the table is populated before traffic arrives.
func (w *watcher) run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.refresh(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.refresh(ctx)
		}
	}
}

// refresh lists Environments and atomically replaces the routing table. Errors
// are logged but never fatal: a transient API failure must not take the proxy
// down, and the previous table remains in place until the next successful poll.
func (w *watcher) refresh(ctx context.Context) {
	list, err := w.client.PinchyV1beta1().Environments(w.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		w.logger.Error("failed to list environments", "error", err)
		return
	}

	routes := make(map[string]string, len(list.Items))
	for i := range list.Items {
		env := &list.Items[i]

		// Only route to running environments that have been assigned a Pod IP.
		if env.Status.Phase != v1beta1.EnvironmentPhaseRunning {
			continue
		}

		if env.Status.PodIP == "" {
			continue
		}

		routes[env.Name] = env.Status.PodIP
	}

	w.table.replace(routes)
	w.logger.Debug("routing table updated", "routes", len(routes))
}
