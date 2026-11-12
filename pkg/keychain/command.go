package keychain

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RunCommand runs command with "sh -c" ("cmd /C" on Windows) and returns its output, trimmed of
// surrounding whitespace. An empty output is an error. stdin may be nil.
func RunCommand(ctx context.Context, command string, stdin io.Reader) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	// Children of the shell may keep stdout open after a timeout kill; don't wait for them forever.
	cmd.WaitDelay = time.Second

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = stdin
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%q failed: %w: %s", command, err, msg)
		}
		return "", fmt.Errorf("%q failed: %w", command, err)
	}

	secret := strings.TrimSpace(string(out))
	if secret == "" {
		return "", fmt.Errorf("%q printed nothing", command)
	}
	return secret, nil
}

// ExpandCommand replaces {namespace} and {path} in a command template, e.g.
// "pass show {namespace}/{path}" becomes "pass show ghcli/github.pat". A template without
// placeholders is returned unchanged, so it can serve a single secret.
func ExpandCommand(template, namespace, path string) string {
	return strings.NewReplacer("{namespace}", namespace, "{path}", path).Replace(template)
}
