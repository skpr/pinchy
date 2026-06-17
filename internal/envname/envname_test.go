package envname_test

import (
	"strings"
	"testing"

	"github.com/skpr/pinchy/internal/envname"
)

func TestFromPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantEmpty   bool
		wantPrefix  string
		sameAs      string // if non-empty, result must equal envname.FromPath(sameAs)
		differentTo string // if non-empty, result must differ from envname.FromPath(differentTo)
	}{
		{
			name:      "empty path returns empty string",
			path:      "",
			wantEmpty: true,
		},
		{
			name:       "normal path has env- prefix",
			path:       "/home/user/project",
			wantPrefix: "env-",
		},
		{
			name:   "trailing slash is normalised away",
			path:   "/home/user/project/",
			sameAs: "/home/user/project",
		},
		{
			name:   "double slash is normalised",
			path:   "/home/user//project",
			sameAs: "/home/user/project",
		},
		{
			name:   "case is normalised",
			path:   "/Home/User/Project",
			sameAs: "/home/user/project",
		},
		{
			name:        "different paths produce different names",
			path:        "/home/user/project-a",
			differentTo: "/home/user/project-b",
		},
		{
			name:       "result is always 16 chars (env- + 12 hex)",
			path:       "/any/path/here",
			wantPrefix: "env-",
		},
		{
			name:   "dot-dot is resolved",
			path:   "/home/user/project/../project",
			sameAs: "/home/user/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envname.FromPath(tt.path)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("FromPath(%q) = %q, want empty", tt.path, got)
				}
				return
			}

			if got == "" {
				t.Fatalf("FromPath(%q) returned empty, want non-empty", tt.path)
			}

			if tt.wantPrefix != "" && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("FromPath(%q) = %q, want prefix %q", tt.path, got, tt.wantPrefix)
			}

			// Verify fixed length: "env-" (4) + 12 hex chars = 16
			if tt.path != "" && len(got) != 16 {
				t.Errorf("FromPath(%q) = %q (len %d), want len 16", tt.path, got, len(got))
			}

			if tt.sameAs != "" {
				want := envname.FromPath(tt.sameAs)
				if got != want {
					t.Errorf("FromPath(%q) = %q, want same as FromPath(%q) = %q", tt.path, got, tt.sameAs, want)
				}
			}

			if tt.differentTo != "" {
				other := envname.FromPath(tt.differentTo)
				if got == other {
					t.Errorf("FromPath(%q) = %q, want different from FromPath(%q) = %q", tt.path, got, tt.differentTo, other)
				}
			}
		})
	}
}

func TestFromPath_RFC1123Valid(t *testing.T) {
	paths := []string{
		"/home/user/my-project",
		"/var/lib/data",
		"/Users/Alice/Code/Repo",
		"/srv/workspace with spaces",
		"/tmp/a",
	}

	for _, path := range paths {
		name := envname.FromPath(path)
		if name == "" {
			t.Errorf("FromPath(%q) returned empty", path)
			continue
		}
		// Must start and end with alphanumeric.
		first := name[0]
		last := name[len(name)-1]
		if !isAlphanumeric(first) || !isAlphanumeric(last) {
			t.Errorf("FromPath(%q) = %q: must start and end with alphanumeric", path, name)
		}
		// Must contain only lowercase alphanumeric and hyphens.
		for _, c := range name {
			if !isAlphanumeric(byte(c)) && c != '-' {
				t.Errorf("FromPath(%q) = %q: invalid char %q", path, name, c)
			}
		}
	}
}

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
