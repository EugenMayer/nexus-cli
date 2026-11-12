package auth

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EugenMayer/nexus-cli/internal/cli"
	"github.com/EugenMayer/nexus-cli/internal/config"
	"github.com/EugenMayer/nexus-cli/internal/nexus"
)

func newLoginCommand() *cli.Command {
	var (
		url          string
		user         string
		withPassword bool
		timeout      time.Duration
	)
	return &cli.Command{
		Name:    "login",
		Summary: "Store the username and password in the OS keyring",
		Long: "Asks for the username (unless -user is given) and the password, with\n" +
			"hidden input, and stores both in the OS keyring. With -url the\n" +
			"credentials are checked against that instance first and only stored\n" +
			"if it accepts them.\n\n" +
			"  nexus auth login -url https://nexus.example.com\n" +
			"  pass show work/nexus | nexus auth login -user admin -with-password",
		Flags: func(fs *flag.FlagSet) {
			fs.StringVar(&url, "url", "", "check the credentials against this Nexus base URL before storing them")
			fs.StringVar(&user, "user", "", "username (prompted for if omitted)")
			fs.BoolVar(&withPassword, "with-password", false, "read the password from standard input")
			fs.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP timeout for the -url check")
		},
		Run: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
			}
			if user == "" {
				if withPassword || !isTerminal(stdin) {
					return errors.New("-user is required when the password is read from standard input")
				}
				fmt.Fprint(stderr, "Username: ")
				line, err := bufio.NewReader(stdin).ReadString('\n')
				if user = strings.TrimSpace(line); user == "" {
					return fmt.Errorf("no username given: %v", err)
				}
			}

			pass, err := ReadSecret(stdin, stderr, fmt.Sprintf("Password for %s: ", user), withPassword)
			if err != nil {
				return err
			}

			if url != "" {
				if err := nexus.New(url, user, pass, timeout).CheckAuth(ctx); err != nil {
					return fmt.Errorf("login to %s failed, nothing stored: %w", url, err)
				}
				fmt.Fprintf(stdout, "%s accepted the credentials\n", url)
			}

			kc := config.Keychain
			if err := kc.Set(config.UserPath, user); err != nil {
				return err
			}
			if err := kc.Set(config.PassPath, pass); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "stored the credentials of %q in the OS keyring\n", user)

			for _, env := range []string{"NEXUS_USER", "NEXUS_PASS"} {
				if os.Getenv(env) != "" {
					fmt.Fprintf(stderr, "warning: $%s is set and takes precedence over the OS keyring\n", env)
				}
			}
			return nil
		},
	}
}
