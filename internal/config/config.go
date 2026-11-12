// Package config holds connection settings shared by every command that talks
// to a Nexus instance, so subcommands register the same -url/-repo/credential
// flags in a consistent way.
package config

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/EugenMayer/nexus-cli/internal/nexus"
	"github.com/EugenMayer/nexus-cli/pkg/keychain"
)

// Keychain holds nexus' secrets in the OS keyring, under namespace "nexus".
var Keychain = keychain.New("nexus")

// Keyring paths of the credentials stored by "nexus auth login". Their
// environment variables are NEXUS_USER and NEXUS_PASS (see keychain.DefaultEnvVar).
const (
	UserPath = "user"
	PassPath = "pass"
)

// Connection is the set of flags needed to reach a Nexus repository.
type Connection struct {
	URL     string
	Repo    string
	User    string
	Pass    string
	Timeout time.Duration
}

// Register binds the connection flags onto fs. Call from a command's Flags hook.
// repoRequired controls whether -repo is documented as required (some future
// commands may be instance-wide rather than per-repository).
func (c *Connection) Register(fs *flag.FlagSet, repoRequired bool) {
	repoHelp := "repository name"
	if repoRequired {
		repoHelp += " (required)"
	}
	c.RegisterServer(fs)
	fs.StringVar(&c.Repo, "repo", "", repoHelp)
}

// RegisterServer binds the flags to reach the instance (-url, credentials,
// -timeout) without -repo, for instance-wide commands.
func (c *Connection) RegisterServer(fs *flag.FlagSet) {
	fs.StringVar(&c.URL, "url", "", "Nexus base URL, e.g. https://nexus.example.com (required)")
	fs.StringVar(&c.User, "user", "", "username (default $NEXUS_USER, then the OS keyring, see \"nexus auth login\")")
	fs.StringVar(&c.Pass, "pass", "", "password or a reference: keychain:<path>, env:<NAME>, cmd:<command>\n"+
		"(default $NEXUS_PASS, then the OS keyring, see \"nexus auth login\")")
	fs.DurationVar(&c.Timeout, "timeout", 30*time.Second, "per-request HTTP timeout")
}

// Validate checks required fields. repoRequired mirrors the Register call.
func (c *Connection) Validate(repoRequired bool) error {
	if c.URL == "" {
		return fmt.Errorf("-url is required")
	}
	if repoRequired && c.Repo == "" {
		return fmt.Errorf("-repo is required")
	}
	return nil
}

// Client builds a nexus.Client from the connection settings, resolving the
// credentials first (see ResolveCredentials).
func (c *Connection) Client(ctx context.Context) (*nexus.Client, error) {
	cr, err := c.ResolveCredentials(ctx)
	if err != nil {
		return nil, err
	}
	return c.ClientWith(cr), nil
}

// ClientWith builds a nexus.Client using already resolved credentials.
func (c *Connection) ClientWith(cr Credentials) *nexus.Client {
	return nexus.New(c.URL, cr.User, cr.Pass, c.Timeout)
}

// Credentials are resolved login credentials. Empty User means anonymous.
type Credentials struct {
	User, Pass string
	// UserFrom and PassFrom describe the source, e.g. "-user", "$NEXUS_PASS"
	// or "OS keyring"; empty if unset.
	UserFrom, PassFrom string
}

// ResolveCredentials resolves the username and password. Both -user and -pass
// may be references like "keychain:work.pass" (see keychain.ResolveRef).
// Otherwise each comes from its environment variable ($NEXUS_USER,
// $NEXUS_PASS), then from the OS keyring as stored by "nexus auth login". The
// keyring password is only used together with a username.
func (c *Connection) ResolveCredentials(ctx context.Context) (Credentials, error) {
	var cr Credentials
	// A keyring failure seen while resolving the user is reused for the
	// password, so an unresponsive keyring isn't waited for twice.
	var keychainErr error

	if c.User != "" {
		v, err := keychain.ResolveRef(ctx, Keychain, c.User)
		if err != nil {
			return cr, fmt.Errorf("-user: %w", err)
		}
		cr.User, cr.UserFrom = v, "-user"
	} else {
		s, err := (&keychain.Resolver{Keychain: Keychain}).Resolve(ctx, UserPath)
		var nf *keychain.NotFoundError
		switch {
		case err == nil:
			cr.User, cr.UserFrom = s.Value, sourceName(s)
		case errors.As(err, &nf):
			if errors.Is(nf.KeychainErr, keychain.ErrUnavailable) {
				keychainErr = nf.KeychainErr
			}
		default:
			return cr, err
		}
	}

	if c.Pass != "" {
		v, err := keychain.ResolveRef(ctx, Keychain, c.Pass)
		if err != nil {
			return cr, fmt.Errorf("-pass: %w", err)
		}
		cr.Pass, cr.PassFrom = v, "-pass"
		return cr, nil
	}

	r := &keychain.Resolver{Keychain: Keychain, KeychainErr: keychainErr}
	if cr.User == "" {
		// anonymous: honor $NEXUS_PASS for compatibility, but skip the keyring
		r.KeychainErr = keychain.ErrNotFound
	}
	s, err := r.Resolve(ctx, PassPath)
	var nf *keychain.NotFoundError
	switch {
	case err == nil:
		cr.Pass, cr.PassFrom = s.Value, sourceName(s)
		return cr, nil
	case cr.User == "" && errors.Is(err, keychain.ErrNotFound):
		return cr, nil
	case errors.As(err, &nf) && errors.Is(nf.KeychainErr, keychain.ErrUnavailable):
		return cr, fmt.Errorf("no password for user %q: pass -pass or set NEXUS_PASS (OS keyring unavailable: %s)",
			cr.User, keychain.UnavailableReason(nf.KeychainErr))
	case errors.Is(err, keychain.ErrNotFound):
		return cr, fmt.Errorf("no password for user %q: run \"nexus auth login\", pass -pass or set NEXUS_PASS",
			cr.User)
	default:
		return cr, err
	}
}

func sourceName(s keychain.Secret) string {
	if s.Source == keychain.SourceEnv {
		return "$" + s.EnvVar
	}
	return s.Source.String()
}
