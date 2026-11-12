package keychain

import (
	"context"

	"github.com/zalando/go-keyring"
)

// MockInit replaces the OS keyring with an in-memory store, for tests of programs using this
// package. The returned function restores the availability and backend checks; the in-memory
// store stays in place.
func MockInit() (restore func()) {
	keyring.MockInit()
	oldAvailable, oldBackend := available, backend
	available = func() error { return nil }
	backend = func(context.Context) (string, error) { return "in-memory mock keyring", nil }
	return func() { available, backend = oldAvailable, oldBackend }
}

// MockUnavailable makes the OS keyring unavailable with reason, e.g. to test fallbacks. The
// returned function restores the previous state.
func MockUnavailable(reason error) (restore func()) {
	return MockAvailability(func() error { return reason })
}

// MockAvailability replaces the availability check that runs before every keyring operation:
// a non-nil error makes the keyring unavailable. Counting its calls tells how often a program
// accesses the keyring. The returned function restores the previous check.
func MockAvailability(check func() error) (restore func()) {
	oldAvailable := available
	available = check
	return func() { available = oldAvailable }
}
