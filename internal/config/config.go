// Package config holds connection settings shared by every command that talks
// to a Nexus instance, so subcommands register the same -url/-repo/credential
// flags in a consistent way.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/EugenMayer/nexus-cli/internal/nexus"
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
	fs.StringVar(&c.URL, "url", "", "Nexus base URL, e.g. https://nexus.example.com (required)")
	fs.StringVar(&c.Repo, "repo", "", repoHelp)
	fs.StringVar(&c.User, "user", os.Getenv("NEXUS_USER"), "username (default $NEXUS_USER)")
	fs.StringVar(&c.Pass, "pass", os.Getenv("NEXUS_PASS"), "password (default $NEXUS_PASS)")
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

// Client builds a nexus.Client from the connection settings.
func (c *Connection) Client() *nexus.Client {
	return nexus.New(c.URL, c.User, c.Pass, c.Timeout)
}
