package keychain

import (
	"context"
	"errors"
	"testing"
)

func TestResolveRef(t *testing.T) {
	t.Cleanup(MockInit())
	kc := New("app")
	_ = kc.Set("jira.token", "from-keychain")
	t.Setenv("JIRA_TOKEN", "from-env")
	ctx := context.Background()

	cases := map[string]string{
		"keychain:jira.token":       "from-keychain",
		"env:JIRA_TOKEN":            "from-env",
		"cmd:echo from-cmd":         "from-cmd",
		"literal:keychain:verbatim": "keychain:verbatim",
		"https://jira.example.com":  "https://jira.example.com",
		"plain value":               "plain value",
	}
	for in, want := range cases {
		if got, err := ResolveRef(ctx, kc, in); err != nil || got != want {
			t.Errorf("ResolveRef(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	if _, err := ResolveRef(ctx, kc, "keychain:missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing keychain ref: %v, want ErrNotFound", err)
	}
	if _, err := ResolveRef(ctx, kc, "env:NOT_SET_ANYWHERE_X"); err == nil {
		t.Error("expected error for unset env ref")
	}

	if !IsRef("keychain:x") || !IsRef("cmd:x") || IsRef("https://x") || IsRef("literal:x") {
		t.Error("IsRef mismatch")
	}
}
