// Package envname provides a canonical function for deriving a Kubernetes
// resource name from a workspace path. Using a single shared implementation
// ensures that the API server (create, exec) and the board always agree on
// which Environment resource corresponds to a given path.
package envname

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

// FromPath returns a stable, RFC 1123-compliant Kubernetes resource name for
// the given workspace path.
//
// Design:
//   - The path is normalised (filepath.Clean, trailing-slash stripped, lowercased)
//     before hashing so minor representational differences (e.g. a trailing "/"
//     from one caller vs. none from another) always produce the same name.
//   - The name is "env-<12-hex-chars of sha256(normalised path)>", which is
//     always exactly 16 characters, well within the 253-char limit, and
//     guaranteed collision-free for any two distinct normalised paths.
//   - When path is empty the function returns an empty string; callers should
//     fall back to a session-based name in that case.
func FromPath(path string) string {
	if path == "" {
		return ""
	}

	normalised := normalisePath(path)

	sum := sha256.Sum256([]byte(normalised))
	return fmt.Sprintf("env-%x", sum[:6]) // 6 bytes → 12 hex chars
}

// normalisePath cleans and lowercases a path so that trivially equivalent
// representations (trailing slash, double separators, mixed case on
// case-insensitive systems) always hash identically.
func normalisePath(path string) string {
	path = filepath.Clean(path)
	path = strings.TrimRight(path, "/")
	path = strings.ToLower(path)
	return path
}
