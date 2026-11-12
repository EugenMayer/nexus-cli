package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EugenMayer/nexus-cli/internal/config"
	"github.com/EugenMayer/nexus-cli/pkg/keychain"
)

func dispatch(in string, args ...string) (string, error) {
	var out strings.Builder
	stdin, stdout, stderr = strings.NewReader(in), &out, &out
	err := New().Dispatch(context.Background(), args, &out, &out)
	return out.String(), err
}

func run(t *testing.T, in string, args ...string) string {
	t.Helper()
	out, err := dispatch(in, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}

// nexusServer accepts admin/good only.
func nexusServer(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "admin" || p != "good-password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestLoginStatusLogout(t *testing.T) {
	t.Cleanup(keychain.MockInit())
	t.Setenv("NEXUS_USER", "")
	t.Setenv("NEXUS_PASS", "")
	url := nexusServer(t)

	if out, err := dispatch("wrong\n", "login", "-url", url, "-user", "admin", "-with-password"); err == nil ||
		!strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("login with a wrong password: %v\n%s", err, out)
	}
	if _, err := config.Keychain.Get(config.PassPath); err == nil {
		t.Error("a rejected login must not store anything")
	}

	out := run(t, "good-password\n", "login", "-url", url, "-user", "admin", "-with-password")
	if !strings.Contains(out, "accepted") || !strings.Contains(out, `stored the credentials of "admin"`) {
		t.Errorf("login: %s", out)
	}

	out = run(t, "", "status", "-url", url)
	for _, want := range []string{"keyring:  in-memory mock keyring", "user:     admin (OS keyring)",
		"password: good…word (OS keyring)", "login:    ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("status misses %q:\n%s", want, out)
		}
	}
	if out, err := dispatch("", "status", "-url", url, "-pass", "wrong"); err == nil || !strings.Contains(out, "login:    failed") {
		t.Errorf("status with a wrong password: %v\n%s", err, out)
	}

	if out := run(t, "", "logout"); !strings.Contains(out, "removed") {
		t.Errorf("logout: %s", out)
	}
	if out := run(t, "", "logout"); !strings.Contains(out, "no credentials") {
		t.Errorf("second logout: %s", out)
	}
	if out := run(t, "", "status"); !strings.Contains(out, "anonymous") {
		t.Errorf("status after logout: %s", out)
	}
}

func TestLoginNeedsUserWithoutTerminal(t *testing.T) {
	t.Cleanup(keychain.MockInit())
	if _, err := dispatch("pw\n", "login", "-with-password"); err == nil || !strings.Contains(err.Error(), "-user") {
		t.Errorf("expected -user error, got %v", err)
	}
}
