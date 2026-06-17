package environment

import (
	"regexp"
	"strings"
)

// matches any run of characters that are NOT lowercase alphanumeric, '-' or '.'
var invalidChars = regexp.MustCompile(`[^a-z0-9.-]+`)

// matches leading/trailing characters that aren't alphanumeric
var trimEdges = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)

// RFC1123Subdomain converts s into a string that satisfies the
// RFC 1123 subdomain rules used by Kubernetes metadata.name:
//   - lower case alphanumeric, '-' or '.'
//   - must start and end with an alphanumeric character
//   - at most 253 characters
func RFC1123Subdomain(s string) string {
	s = strings.ToLower(s)
	// replace any invalid character (incl. '_') with '-'
	s = invalidChars.ReplaceAllString(s, "-")
	// strip any leading/trailing non-alphanumeric chars
	s = trimEdges.ReplaceAllString(s, "")

	if len(s) > 253 {
		s = s[:253]
		// trimming may have left a trailing separator; clean it up
		s = trimEdges.ReplaceAllString(s, "")
	}
	return s
}
