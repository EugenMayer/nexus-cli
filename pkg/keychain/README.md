# keychain

Go package to keep a CLI's secrets in the OS keyring instead of its config file. Secrets are
addressed by a path within an application namespace, e.g. namespace `mytool`, path `jira.token`.

- **Backends:** Secret Service on Linux (GNOME Keyring, KWallet, KeePassXC, …), Keychain on macOS,
  Credential Manager on Windows, via [go-keyring](https://github.com/zalando/go-keyring)
- **Safe without a desktop:** on Linux without a D-Bus session (SSH, servers, WSL) the keyring
  reports itself unavailable instead of hanging; every operation has a timeout
- **Other sources:** environment variables, commands for any secret manager (pass, gopass, gpg,
  age, sops, 1Password, Bitwarden, …), plain-text fallbacks and auto-detection
- **Config references:** config values like `keychain:jira.token` resolve to the secret
- **Commands:** nexus builds `auth login|logout|status` on it, see `cmd/auth`

Ported from [ghcli's pkg/keychain](https://github.com/EugenMayer/ghcli/tree/main/pkg/keychain).
Dependencies: go-keyring and godbus only. Nothing else from nexus-cli is imported, so the folder
can also be copied into another project.

## Install

```sh
go get github.com/EugenMayer/nexus-cli/pkg/keychain
```

## Store and fetch

```go
kc := keychain.New("mytool")

kc.Set("jira.token", token)          // service "mytool.jira.token", account "mytool"
token, err := kc.Get("jira.token")   // errors.Is(err, keychain.ErrNotFound) if not stored
kc.Delete("jira.token")

backend, err := kc.Backend(ctx)      // "GNOME Keyring (Secret Service)", "macOS Keychain", …
// errors.Is(err, keychain.ErrUnavailable) if there is no usable keyring
```

Paths may contain letters, digits and `. _ / -`. Any store can also read an entry directly with
`secret-tool lookup service mytool.jira.token username mytool` (Linux) or
`security find-generic-password -s mytool.jira.token -a mytool -w` (macOS).

## Moving config values into the keychain

Let values in your config file reference secrets instead of holding them:

```yaml
jira:
  url: https://jira.example.com
  token: keychain:jira.token      # from the OS keyring
gitlab:
  token: keychain:gitlab.token
slack:
  webhook: env:SLACK_WEBHOOK      # from an environment variable
github:
  token: cmd:pass show work/github   # output of a command
```

```go
token, err := keychain.ResolveRef(ctx, kc, cfg.Jira.Token)
```

Values without one of these prefixes are returned unchanged, so plain values and URLs keep
working. Use `literal:` for a value that happens to start with a prefix.

Give users a command to store the secrets, e.g. like nexus' `cmd/auth` does:

```sh
nexus auth login -url https://nexus.example.com   # stores paths "user" and "pass"
nexus auth status -url https://nexus.example.com
nexus auth logout
```

## Resolving from several sources

`Resolver` looks a secret up in this order, first match wins:

1. environment variable, `MYTOOL_JIRA_TOKEN` by default (`EnvVar` overrides the name)
2. `Command`, a command template overriding the keyring, e.g. `pass show {namespace}/{path}`;
   a failure is an error, there is no fallback
3. the OS keyring
4. `Plain(path)`, e.g. the old plain-text value from your config, so existing setups keep working
5. auto-detection with `Candidates`, if set

```go
r := &keychain.Resolver{
	Keychain:   kc,
	Command:    cfg.KeyringCommand,                       // optional user override
	Plain:      func(path string) string { return legacy[path] },
	Candidates: keychain.DefaultCandidates(),             // optional auto-detection
	Validate:   func(s string) bool { return len(s) > 20 }, // reject unrelated output
}
s, err := r.Resolve(ctx, "jira.token")
// s.Value, s.Source (SourceEnv, SourceCommand, SourceKeychain, SourcePlain, SourceDetected)
// errors.Is(err, keychain.ErrNotFound); errors.As(err, &nf) for *NotFoundError with the
// keyring's reason
```

When `s.Source` is `SourcePlain`, it's a good moment to recommend moving the value with
`mytool secret set <path>`. When it's `SourceDetected`, persist `s.Command` as the user's command
override so detection doesn't run again.

`DefaultCandidates` tries, for namespace `mytool` and path `jira.token`:

| Tool      | Command                                                  | Tried when                    |
|-----------|----------------------------------------------------------|-------------------------------|
| pass      | `pass show mytool/jira.token \| head -n 1`               | the entry exists              |
| gopass    | `gopass show -o mytool/jira.token`                       | installed                     |
| KWallet   | `kwallet-query -f mytool -r jira.token kdewallet`        | installed                     |
| rbw       | `rbw get mytool.jira.token`                              | installed                     |
| Bitwarden | `bw get password mytool.jira.token`                      | installed, `BW_SESSION` set   |
| 1Password | `op read "op://Private/mytool.jira.token/credential"`    | installed                     |
| gpg       | `gpg --quiet --decrypt ~/.mytool/jira.token.gpg`         | the file exists               |
| age       | `age -d -i ~/.config/age/key.txt ~/.mytool/jira.token.age` | both files exist            |

Pass your own `[]keychain.Candidate` for other conventions. Probes get no stdin and are stopped
after 10 seconds, so a tool waiting for an unlock prompt can't block your program.

## Testing

```go
func TestSomething(t *testing.T) {
	t.Cleanup(keychain.MockInit())                                  // in-memory keyring
	t.Cleanup(keychain.MockUnavailable(errors.New("no D-Bus")))    // or: no keyring at all
	…
}
```
