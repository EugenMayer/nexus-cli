# nexus

An extensible command-line client for [Sonatype Nexus 3](https://help.sonatype.com/repomanager3).

The client is organized as a tree of commands. Today it implements the
`content` group for managing the files inside a repository — currently
`content rm`, which deletes assets by regular expression. The structure is
built so new groups and subcommands (`content upload`, `content ls`, repository
administration, ...) slot in without disturbing what exists.

Dependencies are limited to the OS keyring integration (`go-keyring`,
`godbus`, `golang.org/x/term`).

## Install

Download a binary from the [releases page](../../releases), or build from source:

```sh
go install github.com/EugenMayer/nexus-cli@latest
```

The binary is named `nexus-cli` (release assets: `nexus-cli-<os>-<arch>`).

## Usage

```
nexus <command> [subcommand] [flags] [args]
```

Run `nexus` for the command list, or `nexus <command> -h` for details on any
command.

### Credentials

Log in once and the credentials are kept in the OS keyring (GNOME Keyring,
KWallet, KeePassXC via Secret Service on Linux, Keychain on macOS, Credential
Manager on Windows), so they don't live in your shell history or environment:

```sh
nexus auth login -url https://nexus.example.com   # prompts, tests, then stores
pass show work/nexus | nexus auth login -user admin -with-password
nexus auth status -url https://nexus.example.com  # sources + login test, exit 1 on failure
nexus auth logout
```

`-url` is optional for `login` and `status`; without it the credentials are
stored or shown but not tested.

Commands resolve the username and password in this order:

1. `-user` / `-pass`; `-pass` also takes a reference: `keychain:<path>` (OS
   keyring), `env:<NAME>`, `cmd:<command>` (output of e.g. `pass show
   work/nexus`); prefix `literal:` if the password itself starts with one of
   these
2. the `NEXUS_USER` / `NEXUS_PASS` environment variables
3. the OS keyring, as stored by `nexus auth login` (entries `nexus.user` and
   `nexus.pass`, account `nexus`)

On Linux without a D-Bus session (SSH, servers, CI) the keyring is reported
unavailable instead of hanging; use `NEXUS_USER`/`NEXUS_PASS` or
`-pass cmd:...` there.

### `content rm`

Deletes assets whose path matches any pattern. Patterns are
[Go regular expressions](https://pkg.go.dev/regexp/syntax) matched against the
asset path and are OR'd together. A pattern is **not anchored** unless you
anchor it with `^` and `$`, so `\.log$` matches any path ending in `.log`
while `^logs/` matches anything under `logs/`.

Every run prints the full list of matching assets before deleting anything, so
you always see the effect first. `-dry-run` stops after the preview.

```sh
# Preview everything under a dated log tree (deletes nothing)
nexus content rm -url https://nexus.example.com -repo raw -dry-run '^logs/2024-'

# Delete every .tar.gz anywhere in the repo, with confirmation
nexus content rm -url https://nexus.example.com -repo raw '\.tar\.gz$'

# Non-interactive (for scripts) — skips the y/N prompt
nexus content rm -url https://nexus.example.com -repo raw -yes '^stale/'
```

Quote patterns in your shell so `$`, `\`, and `.` reach the program intact.

## Architecture

```
main.go                     root command; registers top-level groups
internal/cli/               command framework (Command tree + dispatcher)
internal/config/            shared connection flags (-url/-repo/creds), credential resolution
internal/nexus/             Nexus REST client + regexp matcher
pkg/keychain/               OS keyring library (see its README), importable by other projects
cmd/content/                the "content" command group
  content.go                group constructor, registers subcommands
  rm.go                     "content rm"
  util.go                   small shared helpers
cmd/auth/                   the "auth" command group (login/logout/status)
```

`internal/cli` is a minimal, dependency-free framework over the standard
`flag` package. A `Command` has a name, summary, usage/long help, an optional
flag registrar, and either a `Run` function (leaf) or child commands (group).
`Dispatch` walks the arguments, descends into subcommands, parses the resolved
command's flags, and runs it — printing contextual help at every level.

## Adding a command

To add, say, `content upload`:

1. Create `cmd/content/upload.go` with a `newUploadCommand() *cli.Command`
   constructor. Give it a `Name`, `Summary`, `Usage`, `Long`, a `Flags` hook
   (call `conn.Register(fs, true)` to reuse the standard connection flags), and
   a `Run` closure.
2. Register it in `cmd/content/content.go` by adding `newUploadCommand()` to the
   `c.Add(...)` call.

That's the whole change — help, flag parsing, and dispatch are handled by the
framework. To add a brand-new top-level group, create a package under `cmd/`
with a `New() *cli.Command` constructor and register it in `main.go`.

Any command that needs the API adds a few lines in its `Run`:

```go
client, err := conn.Client(ctx) // resolves credentials, incl. the OS keyring
if err != nil {
	return err
}
assets, err := client.ListAssets(ctx, conn.Repo)
```

Extend `internal/nexus` with new client methods (e.g. an upload call) as
commands require them.

## Notes

- **Deletes are irreversible.** Use `-dry-run` first.
- **Disk space is reclaimed asynchronously.** Assets disappear from the
  repository immediately, but blob store space is only freed when the
  *Compact blob store* task runs (scheduled under Nexus admin tasks).
- Commands honor Ctrl-C, stopping cleanly between operations.
- `content rm` filters client-side, so each run enumerates the whole
  repository once. Fine for ad-hoc use; a server-side search would suit very
  large repositories.

## Development

```sh
make test        # go test -race ./...
make vet
make fmt-check
make build       # ./dist/nexus-cli for the current platform
make dist        # all release platforms + checksums.txt into ./dist
```

CI (`.github/workflows/build.yml`) runs the format check, vet, tests and
`make dist` on every push to `main` and on pull requests.

## Releasing

Push a semver tag:

```sh
git tag v1.2.0
git push origin v1.2.0
```

`.github/workflows/release.yml` then checks the tag is `vMAJOR.MINOR.PATCH`
(optionally `-pre`; such tags become GitHub pre-releases), runs the checks,
cross-compiles linux/darwin/windows on amd64/arm64 with the tag as version,
and creates a GitHub release with the binaries, `checksums.txt` and generated
notes. Finally it registers the version with the Go module proxy, so

```sh
go install github.com/EugenMayer/nexus-cli@v1.2.0
go get github.com/EugenMayer/nexus-cli/pkg/keychain@v1.2.0
```

work right away and the version shows up on pkg.go.dev.

## License

MIT — see [LICENSE](LICENSE).
