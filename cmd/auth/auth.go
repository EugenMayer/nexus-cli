// Package auth implements the "auth" command group: storing the Nexus
// credentials in the OS keyring (GNOME Keyring, KWallet, macOS Keychain,
// Windows Credential Manager) and checking them. Each subcommand lives in its
// own file.
package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/EugenMayer/nexus-cli/internal/cli"
	"github.com/EugenMayer/nexus-cli/internal/config"
)

// Replaced in tests.
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// New returns the "auth" command group with all its subcommands registered.
func New() *cli.Command {
	service, account := config.Keychain.Entry(config.PassPath)
	c := &cli.Command{
		Name:    "auth",
		Summary: "Manage the credentials in the OS keyring",
		Long: "Store the Nexus credentials in the OS keyring (GNOME Keyring, KWallet,\n" +
			"macOS Keychain, Windows Credential Manager) instead of flags or\n" +
			"environment variables. The password is stored under service\n" +
			fmt.Sprintf("%q, account %q.\n\n", service, account) +
			"Commands resolve the credentials in this order:\n" +
			"  1. -user / -pass (-pass also takes keychain:<path>, env:<NAME>, cmd:<command>)\n" +
			"  2. the NEXUS_USER / NEXUS_PASS environment variables\n" +
			"  3. the OS keyring, as stored by \"nexus auth login\"",
	}
	c.Add(
		newLoginCommand(),
		newLogoutCommand(),
		newStatusCommand(),
	)
	return c
}

// isTerminal reports whether r is an interactive terminal.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// ReadSecret reads a secret from in. If in is a terminal and fromStdin is
// false, prompt is shown on promptOut and the input is hidden; otherwise the
// first line of in is read.
func ReadSecret(in io.Reader, promptOut io.Writer, prompt string, fromStdin bool) (string, error) {
	var secret string
	if !fromStdin && isTerminal(in) {
		fd := int(in.(*os.File).Fd())
		fmt.Fprint(promptOut, prompt)
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(promptOut)
		if err != nil {
			return "", fmt.Errorf("reading secret: %w", err)
		}
		secret = string(b)
	} else {
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("reading secret from standard input: %w", err)
		}
		secret = line
	}

	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("no secret given")
	}
	return secret, nil
}

// Mask shows only the first and last 4 characters of a secret.
func Mask(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "…" + secret[len(secret)-4:]
}
