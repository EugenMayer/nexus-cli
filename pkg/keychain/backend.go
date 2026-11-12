package keychain

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const secretServiceName = "org.freedesktop.secrets"

// Backend describes the OS keyring in use, e.g. "GNOME Keyring (Secret Service)", "KWallet
// (Secret Service)" or "macOS Keychain". It returns an error if the keyring is not usable.
func (k *Keychain) Backend(ctx context.Context) (string, error) {
	if err := k.Available(); err != nil {
		return "", err
	}
	return backend(ctx)
}

// backend is replaced by MockInit.
var backend = detectBackend

func detectBackend(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "macOS Keychain", nil
	case "windows":
		return "Windows Credential Manager", nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return "", fmt.Errorf("connecting to the D-Bus session bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return "", fmt.Errorf("connecting to the D-Bus session bus: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return "", fmt.Errorf("connecting to the D-Bus session bus: %w", err)
	}
	bus := conn.BusObject()

	var pid uint32
	err = bus.CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, secretServiceName).Store(&pid)
	if err == nil {
		return secretServiceProvider(pid) + " (Secret Service)", nil
	}

	// Not running: it may still be started on demand by D-Bus activation.
	var activatable []string
	if err := bus.CallWithContext(ctx, "org.freedesktop.DBus.ListActivatableNames", 0).Store(&activatable); err == nil &&
		slices.Contains(activatable, secretServiceName) {
		return "Secret Service (not running yet, started on first use)", nil
	}
	return "", fmt.Errorf("%w: no Secret Service provider (GNOME Keyring, KWallet, KeePassXC, …) is running", ErrUnavailable)
}

// secretServiceProvider names the program behind a Secret Service process.
func secretServiceProvider(pid uint32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return fmt.Sprintf("unknown provider (PID %d)", pid)
	}
	comm := strings.TrimSpace(string(data))

	switch {
	case strings.HasPrefix(comm, "gnome-keyring"):
		return "GNOME Keyring"
	case strings.HasPrefix(comm, "kwalletd"), strings.HasPrefix(comm, "ksecretd"):
		return "KWallet"
	case strings.HasPrefix(strings.ToLower(comm), "keepassxc"):
		return "KeePassXC"
	case strings.HasPrefix(comm, "oo7"):
		return "oo7"
	default:
		return comm
	}
}
