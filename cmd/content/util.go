package content

import "strings"

// trimLower trims surrounding whitespace and lowercases s. Used for parsing
// interactive confirmation input.
func trimLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
