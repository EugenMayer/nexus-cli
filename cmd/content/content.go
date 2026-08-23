// Package content implements the "content" command group: operations on the
// files (assets) inside a repository. Each subcommand lives in its own file so
// new operations (upload, ls, ...) can be added without touching the others.
package content

import "github.com/EugenMayer/nexus-cli/internal/cli"

// New returns the "content" command group with all its subcommands registered.
func New() *cli.Command {
	c := &cli.Command{
		Name:    "content",
		Summary: "Manage the files stored in a repository",
		Long: "Operate on the assets (files) inside a Nexus repository:\n" +
			"list them, remove them by pattern, and (later) upload them.",
	}
	c.Add(
		newRmCommand(),
		// Future: newUploadCommand(), newLsCommand(), ...
	)
	return c
}
