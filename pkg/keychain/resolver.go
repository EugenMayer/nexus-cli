package keychain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// Source tells where a resolved secret came from.
type Source int

const (
	SourceEnv      Source = iota + 1 // environment variable
	SourceCommand                    // Resolver.Command
	SourceKeychain                   // OS keyring
	SourcePlain                      // Resolver.Plain, e.g. a plain-text config value
	SourceDetected                   // auto-detected via Resolver.Candidates
)

func (s Source) String() string {
	switch s {
	case SourceEnv:
		return "environment"
	case SourceCommand:
		return "command"
	case SourceKeychain:
		return "OS keyring"
	case SourcePlain:
		return "plain text"
	case SourceDetected:
		return "detected command"
	default:
		return "unknown"
	}
}

// Secret is a resolved secret.
type Secret struct {
	Value  string
	Source Source
	// EnvVar is the variable read for SourceEnv.
	EnvVar string
	// Command is the expanded command run for SourceCommand and SourceDetected. For
	// SourceDetected, persist it (e.g. in your config as the command override) so detection
	// doesn't run again.
	Command string
}

// NotFoundError is returned by Resolver.Resolve when no source has the secret. It matches
// ErrNotFound with errors.Is.
type NotFoundError struct {
	Path string
	// KeychainErr is why the OS keyring had nothing: ErrNotFound, or an error matching
	// ErrUnavailable if it couldn't be used.
	KeychainErr error
}

func (e *NotFoundError) Error() string {
	if errors.Is(e.KeychainErr, ErrUnavailable) {
		return fmt.Sprintf("secret %q not found (%v)", e.Path, e.KeychainErr)
	}
	return fmt.Sprintf("secret %q not found", e.Path)
}

func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

// Resolver looks a secret up in several sources, first match wins:
//
//  1. the environment variable EnvVar(path)
//  2. Command, a command template (see ExpandCommand); if set, a failure is returned as is
//     instead of trying the other sources
//  3. the OS keyring
//  4. Plain(path), e.g. a plain-text value from a config file
//  5. auto-detection with Candidates (see Detect)
//
// All fields except Keychain are optional.
type Resolver struct {
	Keychain *Keychain

	// EnvVar names the environment variable for a path; return "" to skip. If nil,
	// DefaultEnvVar is used.
	EnvVar func(path string) string
	// Command overrides the OS keyring, e.g. "pass show {namespace}/{path}".
	Command string
	// Stdin is passed to Command so it may prompt, e.g. for a gpg passphrase. Defaults to
	// os.Stdin.
	Stdin io.Reader
	// Plain returns a plain-text fallback for path, or "".
	Plain func(path string) string
	// Candidates enables auto-detection when no other source has the secret.
	Candidates []Candidate
	// Validate rejects detected output that is not a valid secret, e.g. checks a token format.
	Validate func(secret string) bool
	// ProbeTimeout bounds each candidate, DefaultProbeTimeout if zero.
	ProbeTimeout time.Duration
	// KeychainErr, if set, is a known keyring failure (e.g. from an earlier access that timed
	// out): the keyring is skipped and treated as failed with this error, instead of waiting for
	// another timeout.
	KeychainErr error
}

// DefaultEnvVar returns NAMESPACE_PATH in upper case with non-alphanumerics replaced by "_",
// e.g. "GHCLI_GITHUB_PAT" for namespace "ghcli" and path "github.pat".
func DefaultEnvVar(namespace, path string) string {
	return strings.ToUpper(nonAlnum.ReplaceAllString(namespace+"_"+path, "_"))
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// Resolve returns the secret at path. If no source has it, the error matches ErrNotFound and is
// a *NotFoundError.
func (r *Resolver) Resolve(ctx context.Context, path string) (Secret, error) {
	if err := ValidatePath(path); err != nil {
		return Secret{}, err
	}
	ns := r.Keychain.Namespace()

	envVar := DefaultEnvVar(ns, path)
	if r.EnvVar != nil {
		envVar = r.EnvVar(path)
	}
	if envVar != "" {
		if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
			return Secret{Value: v, Source: SourceEnv, EnvVar: envVar}, nil
		}
	}

	if r.Command != "" {
		command := ExpandCommand(r.Command, ns, path)
		stdin := r.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		v, err := RunCommand(ctx, command, stdin)
		if err != nil {
			return Secret{}, err
		}
		return Secret{Value: v, Source: SourceCommand, Command: command}, nil
	}

	keychainErr := r.KeychainErr
	if keychainErr == nil {
		var v string
		if v, keychainErr = r.Keychain.Get(path); keychainErr == nil {
			return Secret{Value: v, Source: SourceKeychain}, nil
		}
	}

	if r.Plain != nil {
		if v := strings.TrimSpace(r.Plain(path)); v != "" {
			return Secret{Value: v, Source: SourcePlain}, nil
		}
	}

	if len(r.Candidates) > 0 {
		if command, v, ok := Detect(ctx, r.Candidates, ns, path, r.Validate, r.ProbeTimeout); ok {
			return Secret{Value: v, Source: SourceDetected, Command: command}, nil
		}
	}

	return Secret{}, &NotFoundError{Path: path, KeychainErr: keychainErr}
}
