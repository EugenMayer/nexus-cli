package nexus

import (
	"fmt"
	"regexp"
)

// Matcher reports whether an asset path should be selected.
type Matcher interface {
	Match(path string) bool
}

// regexpMatcher matches a path against any of its compiled regexps (OR).
type regexpMatcher struct{ res []*regexp.Regexp }

func (m regexpMatcher) Match(path string) bool {
	for _, re := range m.res {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// NewMatcher compiles the given patterns as Go regular expressions matched
// against the asset path. Multiple patterns are OR'd together. It validates
// every pattern up front and returns an error on the first invalid one.
func NewMatcher(patterns []string) (Matcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns given")
	}
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q: %w", p, err)
		}
		res = append(res, re)
	}
	return regexpMatcher{res: res}, nil
}
