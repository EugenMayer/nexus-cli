// Command nexus is an extensible command-line client for Sonatype Nexus 3.
//
// It is organized as a tree of commands. Today it implements the "content"
// group for managing the files inside a repository; new groups and
// subcommands (uploads, cleanup, repository administration, ...) can be added
// by registering them on the root command in main.
//
//	nexus content rm -url https://nexus.example.com -repo raw '^logs/2024-'
//
// Run "nexus" or "nexus <command> -h" for help.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/EugenMayer/nexus-cli/cmd/content"
	"github.com/EugenMayer/nexus-cli/internal/cli"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	root := &cli.Command{
		Name:    "nexus",
		Summary: "Extensible command-line client for Sonatype Nexus 3",
		Long: "A command-line client for Sonatype Nexus 3, organized as a tree\n" +
			"of commands. Global credentials for any command may be supplied\n" +
			"via -user/-pass flags or the NEXUS_USER/NEXUS_PASS environment\n" +
			"variables.",
	}

	// Register top-level command groups here.
	root.Add(
		content.New(),
		versionCommand(),
	)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := root.Dispatch(ctx, args, os.Stdout, os.Stderr)
	switch {
	case err == nil:
		return 0
	case isUsageError(err):
		return 2
	default:
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
}

func isUsageError(err error) bool {
	var ue *cli.UsageError
	return errors.As(err, &ue)
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:    "version",
		Summary: "Print the version and exit",
		Run: func(context.Context, []string) error {
			fmt.Println("nexus", version)
			return nil
		},
	}
}
