package keychain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Candidate is a command that may hold a secret, tried by Detect.
type Candidate struct {
	// Binary must be on PATH for the candidate to be tried.
	Binary string
	// Command is a template, see ExpandCommand.
	Command string
	// Ready reports whether the candidate is worth running, e.g. because its secret file exists.
	// nil means always.
	Ready func(home, namespace, path string) bool
}

// DefaultProbeTimeout bounds each candidate in Detect, as some tools wait for an unlock prompt.
const DefaultProbeTimeout = 10 * time.Second

// DefaultCandidates returns candidates for common secret managers, using these conventions for
// namespace "app" and path "jira.token":
//
//	pass       pass show app/jira.token | head -n 1   (entry must exist)
//	gopass     gopass show -o app/jira.token
//	KWallet    kwallet-query -f app -r jira.token kdewallet
//	rbw        rbw get app.jira.token
//	Bitwarden  bw get password app.jira.token          (BW_SESSION must be set)
//	1Password  op read "op://Private/app.jira.token/credential"
//	gpg        gpg --quiet --decrypt ~/.app/jira.token.gpg      (file must exist)
//	age        age -d -i ~/.config/age/key.txt ~/.app/jira.token.age  (files must exist)
//
// The OS keyring itself is not a candidate; Resolver checks it before detection.
func DefaultCandidates() []Candidate {
	fileExists := func(parts ...string) func(home, namespace, path string) bool {
		return func(home, namespace, path string) bool {
			for _, p := range parts {
				if !exists(filepath.Join(home, ExpandCommand(p, namespace, path))) {
					return false
				}
			}
			return true
		}
	}
	return []Candidate{
		{Binary: "pass", Command: "pass show {namespace}/{path} | head -n 1",
			Ready: func(home, namespace, path string) bool {
				store := os.Getenv("PASSWORD_STORE_DIR")
				if store == "" {
					store = filepath.Join(home, ".password-store")
				}
				return exists(filepath.Join(store, namespace, path+".gpg"))
			}},
		{Binary: "gopass", Command: "gopass show -o {namespace}/{path}"},
		{Binary: "kwallet-query", Command: "kwallet-query -f {namespace} -r {path} kdewallet"},
		{Binary: "rbw", Command: "rbw get {namespace}.{path}"},
		{Binary: "bw", Command: "bw get password {namespace}.{path}",
			Ready: func(string, string, string) bool { return os.Getenv("BW_SESSION") != "" }},
		{Binary: "op", Command: `op read "op://Private/{namespace}.{path}/credential"`},
		{Binary: "gpg", Command: "gpg --quiet --decrypt ~/.{namespace}/{path}.gpg",
			Ready: fileExists(".{namespace}/{path}.gpg")},
		{Binary: "age", Command: "age -d -i ~/.config/age/key.txt ~/.{namespace}/{path}.age",
			Ready: fileExists(".{namespace}/{path}.age", ".config/age/key.txt")},
	}
}

// Detect tries candidates in order and returns the expanded command of the first one that prints
// a secret accepted by validate (any non-empty output if validate is nil). Candidates whose binary
// is missing or that aren't ready are skipped, as are commands that fail or exceed timeout
// (DefaultProbeTimeout if zero). Commands get no stdin, so they can't block on a terminal prompt.
func Detect(ctx context.Context, candidates []Candidate, namespace, path string,
	validate func(string) bool, timeout time.Duration) (command, secret string, ok bool) {
	if ValidatePath(path) != nil {
		return "", "", false
	}
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	home, _ := os.UserHomeDir()

	for _, c := range candidates {
		if _, err := exec.LookPath(c.Binary); err != nil {
			continue
		}
		if c.Ready != nil && !c.Ready(home, namespace, path) {
			continue
		}

		command := ExpandCommand(c.Command, namespace, path)
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		s, err := RunCommand(probeCtx, command, nil)
		cancel()
		if err == nil && (validate == nil || validate(s)) {
			return command, s, true
		}
	}
	return "", "", false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
