package auth

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/EugenMayer/nexus-cli/internal/cli"
	"github.com/EugenMayer/nexus-cli/internal/config"
	"github.com/EugenMayer/nexus-cli/pkg/keychain"
)

func newStatusCommand() *cli.Command {
	var conn config.Connection
	return &cli.Command{
		Name:    "status",
		Summary: "Show which credentials are used and test them",
		Long: "Shows the keyring backend and the credentials commands would use, with\n" +
			"their source. With -url they are tested against that instance; a\n" +
			"failed login exits with status 1, so this also works in scripts.\n\n" +
			"  nexus auth status -url https://nexus.example.com",
		Flags: func(fs *flag.FlagSet) {
			conn.RegisterServer(fs)
		},
		Run: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
			}

			if backend, err := config.Keychain.Backend(ctx); err != nil {
				fmt.Fprintf(stdout, "keyring:  unavailable: %s\n", keychain.UnavailableReason(err))
			} else {
				fmt.Fprintf(stdout, "keyring:  %s\n", backend)
			}

			cr, err := conn.ResolveCredentials(ctx)
			if err != nil {
				return err
			}
			if cr.User == "" {
				fmt.Fprintln(stdout, "user:     none, anonymous access (run \"nexus auth login\")")
			} else {
				fmt.Fprintf(stdout, "user:     %s (%s)\n", cr.User, cr.UserFrom)
			}
			if cr.Pass != "" {
				fmt.Fprintf(stdout, "password: %s (%s)\n", Mask(cr.Pass), cr.PassFrom)
			}

			if conn.URL == "" {
				fmt.Fprintln(stdout, "login:    not tested, pass -url to test it")
				return nil
			}
			if err := conn.ClientWith(cr).CheckAuth(ctx); err != nil {
				fmt.Fprintf(stdout, "login:    failed\n")
				return fmt.Errorf("login to %s: %w", conn.URL, err)
			}
			fmt.Fprintf(stdout, "login:    ok, %s accepted the credentials\n", conn.URL)
			return nil
		},
	}
}
