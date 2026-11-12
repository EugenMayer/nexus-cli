// Package keychain stores and fetches secrets in the OS keyring, addressed by a path inside an
// application namespace, e.g. namespace "ghcli" and path "github.pat".
//
// Backends: the Secret Service on Linux (GNOME Keyring, KWallet, KeePassXC, …), the Keychain on
// macOS and the Credential Manager on Windows. The package also resolves secrets from environment
// variables, commands (pass, gpg, sops, 1Password, …) and plain-text fallbacks, see Resolver, and
// lets config values reference secrets, see ResolveRef.
//
// It depends on nothing but github.com/zalando/go-keyring and github.com/godbus/dbus/v5, so it
// can be imported or copied into other projects.
package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

var (
	// ErrNotFound is returned when a secret is not stored.
	ErrNotFound = errors.New("secret not found")
	// ErrUnavailable is returned when the OS keyring can't be used, e.g. without a D-Bus session.
	ErrUnavailable = errors.New("OS keyring unavailable")
)

// DefaultTimeout bounds a keyring operation. It is generous, as the desktop may show an unlock
// prompt; without it a keyring that never answers would block forever.
const DefaultTimeout = 30 * time.Second

// Keychain accesses the secrets of one application namespace in the OS keyring.
type Keychain struct {
	namespace string
	// Timeout bounds each keyring operation, DefaultTimeout if zero.
	Timeout time.Duration
}

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// New returns a Keychain for namespace, e.g. the application name. It panics on an invalid
// namespace, as that is a programming error.
func New(namespace string) *Keychain {
	if !validName.MatchString(namespace) {
		panic(fmt.Sprintf("keychain: invalid namespace %q", namespace))
	}
	return &Keychain{namespace: namespace}
}

// Namespace returns the namespace given to New.
func (k *Keychain) Namespace() string { return k.namespace }

// Entry returns the keyring service and account a path is stored under:
// service "<namespace>.<path>", account "<namespace>".
func (k *Keychain) Entry(path string) (service, account string) {
	return k.namespace + "." + path, k.namespace
}

// ValidatePath checks that path is usable: letters, digits and . _ / -, starting with a letter or
// digit. This also keeps paths safe to substitute into command templates.
func ValidatePath(path string) error {
	if !validName.MatchString(path) {
		return fmt.Errorf("invalid secret path %q: use letters, digits and . _ / -", path)
	}
	return nil
}

// Get returns the secret stored at path, ErrNotFound if there is none.
func (k *Keychain) Get(path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	service, account := k.Entry(path)

	var secret string
	err := k.do(func() error {
		var err error
		secret, err = keyring.Get(service, account)
		return err
	})
	if err != nil {
		return "", err
	}
	if secret = strings.TrimSpace(secret); secret == "" {
		return "", ErrNotFound
	}
	return secret, nil
}

// Set stores secret at path, replacing an existing one.
func (k *Keychain) Set(path, secret string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("refusing to store an empty secret")
	}
	service, account := k.Entry(path)
	return k.do(func() error { return keyring.Set(service, account, secret) })
}

// Delete removes the secret at path. It reports false if nothing was stored.
func (k *Keychain) Delete(path string) (bool, error) {
	if err := ValidatePath(path); err != nil {
		return false, err
	}
	service, account := k.Entry(path)
	err := k.do(func() error { return keyring.Delete(service, account) })
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Available reports why the OS keyring can't be used (wrapping ErrUnavailable), or nil.
func (k *Keychain) Available() error {
	if err := available(); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// do runs a keyring operation after the availability check, bounded by the timeout. Errors are
// ErrNotFound or match ErrUnavailable.
func (k *Keychain) do(op func() error) error {
	if err := k.Available(); err != nil {
		return err
	}

	timeout := k.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	done := make(chan error, 1)
	go func() { done <- op() }()

	select {
	case err := <-done:
		switch {
		case err == nil:
			return nil
		case errors.Is(err, keyring.ErrNotFound):
			return ErrNotFound
		default:
			// e.g. a locked collection or a D-Bus failure
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	case <-time.After(timeout):
		return fmt.Errorf("%w: no answer within %s", ErrUnavailable, timeout)
	}
}

// available is replaced by MockInit. On Linux the Secret Service is reached via the D-Bus session
// bus. Without one, the D-Bus library would launch a new bus whose keyring is locked and can't
// show an unlock prompt, which blocks forever (SSH sessions, servers, WSL).
var available = func() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		return nil
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "bus")); err == nil {
			return nil
		}
	}
	return errors.New("no D-Bus session bus (DBUS_SESSION_BUS_ADDRESS is not set)")
}

// UnavailableReason returns the reason of an error matching ErrUnavailable without the
// "OS keyring unavailable: " prefix, e.g. "no D-Bus session bus …", for messages that already
// say the keyring is unavailable.
func UnavailableReason(err error) string {
	return strings.TrimPrefix(err.Error(), ErrUnavailable.Error()+": ")
}
