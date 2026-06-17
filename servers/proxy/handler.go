package proxy

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// domainSuffix is the host suffix the proxy serves. Requests must arrive as
// NAME.pinchy.localhost (optionally with a port) to be routed.
const domainSuffix = ".pinchy.localhost"

// newHandler returns an HTTP handler that reverse-proxies requests to the Pod
// backing the Environment named in the request host. podPort is the port on
// the destination Pod that traffic is forwarded to.
func newHandler(table *routeTable, podPort int, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := environmentNameFromHost(r.Host)
		if !ok {
			http.Error(w, fmt.Sprintf("host must be of the form NAME%s", domainSuffix), http.StatusBadRequest)
			return
		}

		podIP, ok := table.lookup(name)
		if !ok {
			http.Error(w, fmt.Sprintf("no running environment found for %q", name), http.StatusNotFound)
			return
		}

		target := &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(podIP, fmt.Sprintf("%d", podPort)),
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
			logger.Error("upstream request failed", "environment", name, "target", target.String(), "error", err)
			http.Error(rw, "upstream environment unreachable", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	})
}

// environmentNameFromHost extracts the Environment name from a request host.
// It strips any port, validates the .pinchy.localhost suffix, and returns the
// leading label. It returns false when the host does not match the expected
// form or the name segment is empty or contains further dots.
func environmentNameFromHost(host string) (string, bool) {
	if host == "" {
		return "", false
	}

	// Strip the port if present (SplitHostPort fails when no port exists).
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	host = strings.ToLower(strings.TrimSuffix(host, "."))

	if !strings.HasSuffix(host, domainSuffix) {
		return "", false
	}

	name := strings.TrimSuffix(host, domainSuffix)
	if name == "" || strings.Contains(name, ".") {
		return "", false
	}

	return name, true
}
