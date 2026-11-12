package keychain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDefaultEnvVar(t *testing.T) {
	cases := map[[2]string]string{
		{"ghcli", "github.pat"}:   "GHCLI_GITHUB_PAT",
		{"my-tool", "jira/token"}: "MY_TOOL_JIRA_TOKEN",
	}
	for in, want := range cases {
		if got := DefaultEnvVar(in[0], in[1]); got != want {
			t.Errorf("DefaultEnvVar(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestExpandCommand(t *testing.T) {
	if got := ExpandCommand("pass show {namespace}/{path}", "app", "jira.token"); got != "pass show app/jira.token" {
		t.Errorf("got %q", got)
	}
	if got := ExpandCommand("echo fixed", "app", "x"); got != "echo fixed" {
		t.Errorf("got %q", got)
	}
}

func TestResolverPrecedence(t *testing.T) {
	t.Cleanup(MockInit())
	kc := New("app")
	ctx := context.Background()
	plain := func(string) string { return "from-plain" }

	check := func(r *Resolver, wantValue string, wantSource Source) {
		t.Helper()
		s, err := r.Resolve(ctx, "jira.token")
		if err != nil || s.Value != wantValue || s.Source != wantSource {
			t.Errorf("Resolve() = %+v, %v; want %q from %s", s, err, wantValue, wantSource)
		}
	}

	_, err := (&Resolver{Keychain: kc}).Resolve(ctx, "jira.token")
	var nf *NotFoundError
	if !errors.Is(err, ErrNotFound) || !errors.As(err, &nf) || nf.Path != "jira.token" {
		t.Errorf("expected NotFoundError, got %v", err)
	}

	check(&Resolver{Keychain: kc, Plain: plain}, "from-plain", SourcePlain)

	_ = kc.Set("jira.token", "from-keychain")
	check(&Resolver{Keychain: kc, Plain: plain}, "from-keychain", SourceKeychain)

	r := &Resolver{Keychain: kc, Plain: plain, Command: "echo cmd-{namespace}-{path}"}
	check(r, "cmd-app-jira.token", SourceCommand)

	t.Setenv("APP_JIRA_TOKEN", "from-env")
	check(r, "from-env", SourceEnv)

	r.EnvVar = func(string) string { return "" } // env lookup disabled
	check(r, "cmd-app-jira.token", SourceCommand)

	// a failing command override is an error, not a fallback
	if _, err := (&Resolver{Keychain: kc, Command: "exit 3", EnvVar: r.EnvVar}).Resolve(ctx, "jira.token"); err == nil {
		t.Error("expected error for failing command")
	}
}

func TestResolverUnavailableKeychainFallsBack(t *testing.T) {
	t.Cleanup(MockInit())
	old := available
	available = func() error { return errors.New("no D-Bus session bus") }
	t.Cleanup(func() { available = old })

	kc := New("app")
	s, err := (&Resolver{Keychain: kc, Plain: func(string) string { return "p" }}).Resolve(context.Background(), "x")
	if err != nil || s.Source != SourcePlain {
		t.Errorf("Resolve() = %+v, %v", s, err)
	}

	_, err = (&Resolver{Keychain: kc}).Resolve(context.Background(), "x")
	var nf *NotFoundError
	if !errors.As(err, &nf) || !errors.Is(nf.KeychainErr, ErrUnavailable) || !strings.Contains(err.Error(), "D-Bus") {
		t.Errorf("expected NotFoundError with unavailable keychain, got %v", err)
	}
}

// fakeBin isolates HOME and PATH and installs fake binaries: name -> shell script body.
func fakeBin(t *testing.T, binaries map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"sh", "head", "sleep"} {
		path := ""
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			if _, err := os.Stat(filepath.Join(dir, tool)); err == nil {
				path = filepath.Join(dir, tool)
				break
			}
		}
		if path == "" {
			t.Fatalf("%s not found", tool)
		}
		if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range binaries {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	t.Setenv("PASSWORD_STORE_DIR", "")
	t.Setenv("BW_SESSION", "")
	return home
}

func TestDetectDefaultCandidates(t *testing.T) {
	home := fakeBin(t, map[string]string{
		"gopass": `[ "$3" = app/jira.token ] && echo not-valid`, // rejected by validate
		"pass":   `printf 'tok-pass\nlogin: me\n'`,              // not ready: no store entry yet
		"rbw":    `[ "$2" = app.jira.token ] && echo tok-rbw`,
	})
	validate := func(s string) bool { return strings.HasPrefix(s, "tok-") }

	command, secret, ok := Detect(context.Background(), DefaultCandidates(), "app", "jira.token", validate, 0)
	if !ok || command != "rbw get app.jira.token" || secret != "tok-rbw" {
		t.Fatalf("Detect() = %q, %q, %v", command, secret, ok)
	}

	if err := os.MkdirAll(filepath.Join(home, ".password-store", "app"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".password-store", "app", "jira.token.gpg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	command, secret, ok = Detect(context.Background(), DefaultCandidates(), "app", "jira.token", validate, 0)
	if !ok || command != "pass show app/jira.token | head -n 1" || secret != "tok-pass" {
		t.Fatalf("Detect() = %q, %q, %v", command, secret, ok)
	}
}

func TestDetectTimeout(t *testing.T) {
	fakeBin(t, map[string]string{"slow": "sleep 5; echo late"})
	start := time.Now()
	_, _, ok := Detect(context.Background(), []Candidate{{Binary: "slow", Command: "slow"}}, "app", "x", nil, 100*time.Millisecond)
	if ok {
		t.Error("expected no detection for a hanging command")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Detect took %s, timeout not applied", d)
	}
}

func TestResolverDetects(t *testing.T) {
	t.Cleanup(MockInit())
	fakeBin(t, map[string]string{"rbw": "echo tok-detected"})

	r := &Resolver{Keychain: New("app"), Candidates: DefaultCandidates()}
	s, err := r.Resolve(context.Background(), "jira.token")
	if err != nil || s.Source != SourceDetected || s.Value != "tok-detected" || s.Command != "rbw get app.jira.token" {
		t.Errorf("Resolve() = %+v, %v", s, err)
	}
}

func TestResolverKnownKeychainErrSkipsKeychain(t *testing.T) {
	t.Cleanup(MockInit())
	kc := New("app")
	_ = kc.Set("x", "from-keychain")

	calls := 0
	t.Cleanup(MockAvailability(func() error { calls++; return nil }))

	known := fmt.Errorf("%w: no answer within 30s", ErrUnavailable)
	s, err := (&Resolver{Keychain: kc, KeychainErr: known, Plain: func(string) string { return "p" }}).Resolve(context.Background(), "x")
	if err != nil || s.Source != SourcePlain || calls != 0 {
		t.Errorf("Resolve() = %+v, %v, keyring accessed %d times; want plain, no access", s, err, calls)
	}
}
