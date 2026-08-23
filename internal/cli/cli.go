// Package cli is a tiny, dependency-free command framework for building a
// tree of (sub)commands on top of the standard library's flag package.
//
// A Command has a name, a one-line summary, optional long help, an optional
// flag registrar, and either a Run function (a leaf command) or child
// commands (a group), or both. Dispatch walks os.Args, descending into
// subcommands, until it reaches a command with a Run function, then parses
// that command's flags and invokes it.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// Command is a node in the command tree.
type Command struct {
	// Name is the single token used to invoke this command, e.g. "rm".
	Name string
	// Summary is a one-line description shown in parent listings.
	Summary string
	// Usage is the argument spec shown after the command path in help,
	// e.g. "PATTERN [PATTERN...]". May be empty.
	Usage string
	// Long is optional extended help printed under the usage line.
	Long string

	// Flags, if set, registers command-specific flags on fs. It is called
	// before parsing. Bind flag values to fields your Run closure captures.
	Flags func(fs *flag.FlagSet)

	// Run executes a leaf command with the post-flag positional args. If Run
	// is nil the command is a pure group and must have Subcommands.
	Run func(ctx context.Context, args []string) error

	// Subcommands are child commands, keyed by name at registration.
	subcommands map[string]*Command
	order       []string
	parent      *Command
}

// Add registers one or more child commands and returns the receiver so calls
// can be chained. It panics on duplicate names, since that is a programming
// error discovered at startup.
func (c *Command) Add(children ...*Command) *Command {
	if c.subcommands == nil {
		c.subcommands = map[string]*Command{}
	}
	for _, child := range children {
		if _, dup := c.subcommands[child.Name]; dup {
			panic(fmt.Sprintf("cli: duplicate subcommand %q under %q", child.Name, c.Name))
		}
		child.parent = c
		c.subcommands[child.Name] = child
		c.order = append(c.order, child.Name)
	}
	return c
}

// path returns the space-joined command path from the root, e.g. "nexus content rm".
func (c *Command) path() string {
	var parts []string
	for cur := c; cur != nil; cur = cur.parent {
		parts = append([]string{cur.Name}, parts...)
	}
	return strings.Join(parts, " ")
}

// UsageError is returned by Dispatch when the user's invocation is malformed
// (unknown command, missing subcommand, bad flags). The caller should exit 2.
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

func usagef(format string, a ...any) *UsageError {
	return &UsageError{msg: fmt.Sprintf(format, a...)}
}

// Dispatch resolves args against the command tree rooted at c, parses flags for
// the resolved command, and runs it. out/errOut receive help and error text.
//
// Returns nil on success, a *UsageError for malformed invocations (exit 2), or
// whatever the command's Run returns (exit 1).
func (c *Command) Dispatch(ctx context.Context, args []string, out, errOut io.Writer) error {
	cmd := c

	// Descend through subcommand tokens. A leading token that is a flag
	// (starts with '-') stops descent so the current command parses it.
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") && cmd.subcommands != nil {
		next, ok := cmd.subcommands[args[0]]
		if !ok {
			// Only error if this command can't run on its own.
			if cmd.Run == nil {
				printCommandHelp(errOut, cmd)
				return usagef("unknown command %q for %q", args[0], cmd.path())
			}
			break
		}
		cmd = next
		args = args[1:]
	}

	// Handle a bare group (no Run): show its help.
	if cmd.Run == nil {
		printCommandHelp(out, cmd)
		if len(args) > 0 {
			return usagef("%q requires a subcommand", cmd.path())
		}
		return nil
	}

	// Parse this command's flags.
	fs := flag.NewFlagSet(cmd.path(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	var help bool
	fs.BoolVar(&help, "h", false, "show help for this command")
	fs.BoolVar(&help, "help", false, "show help for this command")
	if cmd.Flags != nil {
		cmd.Flags(fs)
	}
	fs.Usage = func() { printCommandHelp(errOut, cmd) }

	if err := fs.Parse(args); err != nil {
		// flag already printed the error and usage.
		return usagef("%v", err)
	}
	if help {
		printCommandHelp(out, cmd)
		return nil
	}

	return cmd.Run(ctx, fs.Args())
}

// printCommandHelp writes contextual help for cmd: its usage line, long help,
// its flags, and its subcommands.
func printCommandHelp(w io.Writer, cmd *Command) {
	usage := cmd.path()
	if cmd.subcommands != nil {
		usage += " <command>"
	}
	if cmd.Usage != "" {
		usage += " " + cmd.Usage
	}
	fmt.Fprintf(w, "usage: %s\n", usage)

	if cmd.Summary != "" {
		fmt.Fprintf(w, "\n%s\n", cmd.Summary)
	}
	if cmd.Long != "" {
		fmt.Fprintf(w, "\n%s\n", strings.TrimRight(cmd.Long, "\n"))
	}

	// Subcommands.
	if len(cmd.order) > 0 {
		fmt.Fprintf(w, "\ncommands:\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		names := append([]string(nil), cmd.order...)
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(tw, "  %s\t%s\n", name, cmd.subcommands[name].Summary)
		}
		tw.Flush()
	}

	// Flags. Build a throwaway FlagSet to render defaults.
	if cmd.Flags != nil {
		fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
		fs.SetOutput(w)
		cmd.Flags(fs)
		has := false
		fs.VisitAll(func(*flag.Flag) { has = true })
		if has {
			fmt.Fprintf(w, "\nflags:\n")
			fs.PrintDefaults()
		}
	}

	if cmd.subcommands != nil {
		fmt.Fprintf(w, "\nRun \"%s <command> -h\" for details on a command.\n", cmd.path())
	}
}
