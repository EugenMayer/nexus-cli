package keychain

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeychain(t *testing.T) {
	t.Cleanup(MockInit())
	kc := New("app")

	if service, account := kc.Entry("github.pat"); service != "app.github.pat" || account != "app" {
		t.Errorf("Entry() = %q, %q", service, account)
	}

	if _, err := kc.Get("jira.token"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() of missing secret: %v, want ErrNotFound", err)
	}
	if err := kc.Set("jira.token", " s3cret \n"); err != nil {
		t.Fatal(err)
	}
	if got, err := kc.Get("jira.token"); err != nil || got != "s3cret" {
		t.Errorf("Get() = %q, %v", got, err)
	}
	// stored under the documented entry
	if got, err := keyring.Get("app.jira.token", "app"); err != nil || got != " s3cret \n" {
		t.Errorf("raw entry = %q, %v", got, err)
	}

	if err := kc.Set("jira.token", "  "); err == nil {
		t.Error("expected error for an empty secret")
	}
	if deleted, err := kc.Delete("jira.token"); err != nil || !deleted {
		t.Errorf("Delete() = %v, %v", deleted, err)
	}
	if deleted, err := kc.Delete("jira.token"); err != nil || deleted {
		t.Errorf("second Delete() = %v, %v; want false, nil", deleted, err)
	}
}

func TestValidatePath(t *testing.T) {
	for _, ok := range []string{"github.pat", "jira/token", "a-b_c.d", "x"} {
		if err := ValidatePath(ok); err != nil {
			t.Errorf("ValidatePath(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".hidden", "with space", "a;rm -rf", "$(x)", "a'b"} {
		if err := ValidatePath(bad); err == nil {
			t.Errorf("ValidatePath(%q): expected error", bad)
		}
	}
}

func TestUnavailable(t *testing.T) {
	t.Cleanup(MockInit())
	old := available
	available = func() error { return errors.New("no D-Bus session bus") }
	t.Cleanup(func() { available = old })

	kc := New("app")
	if _, err := kc.Get("x"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Get() = %v, want ErrUnavailable", err)
	}
	if err := kc.Set("x", "y"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Set() = %v, want ErrUnavailable", err)
	}
	if _, err := kc.Backend(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Backend() = %v, want ErrUnavailable", err)
	}
}

func TestBackendErrorsCountAsUnavailable(t *testing.T) {
	t.Cleanup(MockInit())
	keyring.MockInitWithError(errors.New("failed to unlock correct collection"))
	t.Cleanup(func() { keyring.MockInit() })

	if _, err := New("app").Get("x"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Get() = %v, want ErrUnavailable", err)
	}
}
