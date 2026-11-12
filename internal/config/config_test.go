package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/EugenMayer/nexus-cli/pkg/keychain"
)

func TestCredentials(t *testing.T) {
	t.Cleanup(keychain.MockInit())
	t.Setenv("NEXUS_USER", "")
	t.Setenv("NEXUS_PASS", "")
	t.Setenv("WORK_PASS", "from-env-ref")
	ctx := context.Background()

	check := func(c Connection, wantUser, wantPass string) {
		t.Helper()
		cr, err := c.ResolveCredentials(ctx)
		if err != nil || cr.User != wantUser || cr.Pass != wantPass {
			t.Errorf("ResolveCredentials(%+v) = %+v, %v; want %q, %q", c, cr, err, wantUser, wantPass)
		}
	}

	check(Connection{}, "", "")

	_, err := (&Connection{User: "admin"}).ResolveCredentials(ctx)
	if err == nil || !strings.Contains(err.Error(), "nexus auth login") {
		t.Errorf("missing password: %v", err)
	}

	_ = Keychain.Set(PassPath, "from-keyring")
	_ = Keychain.Set("work.pass", "work-keyring")
	check(Connection{User: "admin"}, "admin", "from-keyring")
	check(Connection{}, "", "") // anonymous ignores the keyring
	check(Connection{User: "admin", Pass: "keychain:work.pass"}, "admin", "work-keyring")
	check(Connection{User: "admin", Pass: "env:WORK_PASS"}, "admin", "from-env-ref")
	check(Connection{User: "admin", Pass: "plain"}, "admin", "plain")

	// user stored by "nexus auth login"
	_ = Keychain.Set(UserPath, "stored-user")
	cr, err := (&Connection{}).ResolveCredentials(ctx)
	if err != nil || cr.User != "stored-user" || cr.Pass != "from-keyring" || cr.UserFrom != "OS keyring" {
		t.Errorf("stored credentials = %+v, %v", cr, err)
	}

	t.Setenv("NEXUS_USER", "env-user")
	t.Setenv("NEXUS_PASS", "from-env")
	cr, err = (&Connection{}).ResolveCredentials(ctx)
	if err != nil || cr.User != "env-user" || cr.Pass != "from-env" || cr.UserFrom != "$NEXUS_USER" || cr.PassFrom != "$NEXUS_PASS" {
		t.Errorf("env credentials = %+v, %v", cr, err)
	}

	_, _ = Keychain.Delete(UserPath)
	t.Setenv("NEXUS_USER", "")
	check(Connection{}, "", "from-env")
}

func TestCredentialsKeyringUnavailable(t *testing.T) {
	t.Cleanup(keychain.MockUnavailable(errors.New("no D-Bus session bus")))
	t.Setenv("NEXUS_PASS", "")

	_, err := (&Connection{User: "admin"}).ResolveCredentials(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no D-Bus session bus") {
		t.Errorf("expected the keyring's reason, got %v", err)
	}
}
