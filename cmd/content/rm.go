package content

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/EugenMayer/nexus-cli/internal/cli"
	"github.com/EugenMayer/nexus-cli/internal/config"
	"github.com/EugenMayer/nexus-cli/internal/nexus"
)

// newRmCommand builds "content rm".
func newRmCommand() *cli.Command {
	var (
		conn   config.Connection
		dryRun bool
		yes    bool
	)

	return &cli.Command{
		Name:    "rm",
		Summary: "Delete assets matching one or more regular expressions",
		Usage:   "PATTERN [PATTERN...]",
		Long: "Deletes assets whose path matches any PATTERN. Patterns are Go\n" +
			"regular expressions matched against the asset path and are OR'd\n" +
			"together. A pattern is not anchored unless you anchor it with ^\n" +
			"and $, so '\\.log$' matches any path ending in .log while '^logs/'\n" +
			"matches anything under logs/.\n\n" +
			"Every run prints the full list of matching assets before deleting\n" +
			"anything. Use -dry-run to stop after that preview.\n\n" +
			"examples:\n" +
			"  nexus content rm -url URL -repo raw -dry-run '^logs/2024-'\n" +
			"  nexus content rm -url URL -repo raw '\\.tar\\.gz$'\n" +
			"  nexus content rm -url URL -repo raw -yes '^stale/'",
		Flags: func(fs *flag.FlagSet) {
			conn.Register(fs, true)
			fs.BoolVar(&dryRun, "dry-run", false, "preview matches without deleting")
			fs.BoolVar(&yes, "yes", false, "skip the confirmation prompt")
		},
		Run: func(ctx context.Context, args []string) error {
			if err := conn.Validate(true); err != nil {
				return err
			}
			if len(args) == 0 {
				return fmt.Errorf("at least one PATTERN is required")
			}
			matcher, err := nexus.NewMatcher(args)
			if err != nil {
				return err
			}

			client := conn.Client()
			fmt.Fprintln(os.Stderr, "scanning repository...")
			assets, err := client.ListAssets(ctx, conn.Repo)
			if err != nil {
				return err
			}

			var matched []nexus.Asset
			for _, a := range assets {
				if matcher.Match(a.Path) {
					matched = append(matched, a)
				}
			}
			sort.Slice(matched, func(i, j int) bool {
				return matched[i].Path < matched[j].Path
			})

			if len(matched) == 0 {
				fmt.Printf("no assets matched (%d scanned)\n", len(assets))
				return nil
			}
			fmt.Println("The following assets match:")
			for _, a := range matched {
				fmt.Printf("  %s\n", a.Path)
			}
			fmt.Printf("\n%d of %d assets matched.\n", len(matched), len(assets))

			if dryRun {
				fmt.Println("(dry-run: nothing deleted)")
				return nil
			}
			if !yes && !confirm(len(matched)) {
				fmt.Println("aborted.")
				return nil
			}

			var failed int
			for _, a := range matched {
				if ctx.Err() != nil {
					fmt.Fprintln(os.Stderr, "interrupted; stopping.")
					break
				}
				if err := client.DeleteAsset(ctx, a.ID); err != nil {
					fmt.Fprintf(os.Stderr, "FAILED %s: %v\n", a.Path, err)
					failed++
					continue
				}
				fmt.Println("deleted", a.Path)
			}

			fmt.Printf("\ndone: %d deleted, %d failed\n", len(matched)-failed, failed)
			if failed > 0 {
				return fmt.Errorf("%d deletions failed", failed)
			}
			return nil
		},
	}
}

func confirm(n int) bool {
	fmt.Printf("Delete these %d assets? [y/N] ", n)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	s := trimLower(line)
	return s == "y" || s == "yes"
}
