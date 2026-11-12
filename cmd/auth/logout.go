package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/EugenMayer/nexus-cli/internal/cli"
	"github.com/EugenMayer/nexus-cli/internal/config"
)

func newLogoutCommand() *cli.Command {
	return &cli.Command{
		Name:    "logout",
		Summary: "Remove the credentials from the OS keyring",
		Run: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
			}
			removed := false
			for _, path := range []string{config.UserPath, config.PassPath} {
				deleted, err := config.Keychain.Delete(path)
				if err != nil {
					return err
				}
				removed = removed || deleted
			}
			if removed {
				fmt.Fprintln(stdout, "removed the credentials from the OS keyring")
			} else {
				fmt.Fprintln(stdout, "no credentials stored in the OS keyring")
			}
			return nil
		},
	}
}
