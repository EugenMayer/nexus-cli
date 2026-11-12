package keychain

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ResolveRef resolves a config value that may reference a secret instead of holding it:
//
//	keychain:<path>   the secret at path in k, e.g. "keychain:jira.token"
//	env:<NAME>        the environment variable NAME
//	cmd:<command>     the output of command, e.g. "cmd:pass show work/jira"
//	literal:<value>   value as is, for values that start with one of these prefixes
//	anything else     returned unchanged
//
// This lets a config file keep its structure while secrets move to the OS keyring:
//
//	jira:
//	  url: https://jira.example.com
//	  token: keychain:jira.token
func ResolveRef(ctx context.Context, k *Keychain, value string) (string, error) {
	prefix, rest, ok := strings.Cut(value, ":")
	if !ok {
		return value, nil
	}

	switch prefix {
	case "keychain":
		secret, err := k.Get(rest)
		if err != nil {
			return "", fmt.Errorf("%s: %w", value, err)
		}
		return secret, nil
	case "env":
		v, ok := os.LookupEnv(rest)
		if !ok || strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("%s: environment variable %s is not set", value, rest)
		}
		return strings.TrimSpace(v), nil
	case "cmd":
		return RunCommand(ctx, rest, os.Stdin)
	case "literal":
		return rest, nil
	default:
		// e.g. "https://…" or "a:b"
		return value, nil
	}
}

// IsRef reports whether value is a reference ResolveRef resolves (keychain:, env: or cmd:).
func IsRef(value string) bool {
	prefix, _, ok := strings.Cut(value, ":")
	return ok && (prefix == "keychain" || prefix == "env" || prefix == "cmd")
}
