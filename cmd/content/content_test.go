package content

import (
	"strings"
	"testing"
)

func TestNewRegistersRm(t *testing.T) {
	grp := New()
	if grp.Name != "content" {
		t.Fatalf("group name = %q, want content", grp.Name)
	}
	// Dispatch help for the group and confirm rm is listed.
	var out, errOut strings.Builder
	if err := grp.Dispatch(nil, []string{"-h"}, &out, &errOut); err != nil {
		// bare group prints help and returns nil; a -h is fine too.
	}
	help := out.String() + errOut.String()
	if !strings.Contains(help, "rm") {
		t.Errorf("group help does not mention rm subcommand:\n%s", help)
	}
}

func TestRmRequiresPatternAndURL(t *testing.T) {
	grp := New()
	// No -url, no pattern: should fail validation (UsageError or run error).
	var out, errOut strings.Builder
	err := grp.Dispatch(nil, []string{"rm"}, &out, &errOut)
	if err == nil {
		t.Error("expected error when -url and PATTERN are missing")
	}
}

func TestTrimLower(t *testing.T) {
	for in, want := range map[string]string{
		"  Y \n": "y",
		"YES":    "yes",
		"n":      "n",
	} {
		if got := trimLower(in); got != want {
			t.Errorf("trimLower(%q) = %q, want %q", in, got, want)
		}
	}
}
