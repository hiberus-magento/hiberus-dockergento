package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Collecting what abandoned environments left behind.
//
// Looking is the default and deleting is asked for, that way round on purpose: a dry run somebody
// has to remember to type protects the people who were already being careful.
func clean(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	remove := os.Getenv("HM_FORCE") == "1"

	for _, argument := range args {
		if argument == "--force" {
			remove = true

			continue
		}

		return failure(stderr, jsonOutput, "clean", exitUsage, "invalid_argument",
			"Unknown option: "+argument, binaryName()+" clean [--force]")
	}

	collection, err := engine(stdout, stderr, jsonOutput).Survey()
	if err != nil {
		return report(stderr, jsonOutput, "clean", err)
	}

	collection.Removed = remove

	if jsonOutput {
		if code := document(stdout, stderr, "clean", collection); code != exitOK {
			return code
		}

		if !remove {
			return exitOK
		}
	} else {
		cleanAsText(collection, stdout)
	}

	if !collection.Anything() {
		return exitOK
	}

	if !remove {
		if !jsonOutput {
			fmt.Fprint(stdout, "  Nothing was deleted. To collect them:\n")
			fmt.Fprint(stdout, warning("  "+binaryName()+" clean --force\n\n"))
		}

		return exitOK
	}

	if !jsonOutput && os.Getenv("HM_NON_INTERACTIVE") == "" {
		if !confirmed(collection, stdout) {
			fmt.Fprint(stdout, good("Nothing was deleted.\n"))

			return exitOK
		}
	}

	if err := engine(stdout, stderr, jsonOutput).Collect(collection); err != nil {
		return report(stderr, jsonOutput, "clean", err)
	}

	if !jsonOutput {
		fmt.Fprint(stdout, good("Done.\n"))
	}

	return exitOK
}

func confirmed(collection core.Collection, stdout io.Writer) bool {
	fmt.Fprintf(stdout, "\n")
	fmt.Fprint(stdout, warning(fmt.Sprintf(
		"This deletes %d environment(s), %d volume(s) and %d branch environment(s).\n",
		len(collection.Environments), len(collection.Volumes), len(collection.Worktrees))))
	fmt.Fprint(stdout, warning("Their database snapshots are not touched.\n\n"))

	answer, err := ask("Delete them? [y/N]:", "")

	return err == nil && strings.EqualFold(strings.TrimSpace(answer), "y")
}

func cleanAsText(collection core.Collection, stdout io.Writer) {
	fmt.Fprintf(stdout, "\n")

	if len(collection.Environments) == 0 {
		fmt.Fprint(stdout, good(
			"Nothing to collect: every environment on this machine still has its directory.\n"))
	} else {
		fmt.Fprintf(stdout, "%s\n", header("Environments whose directory is gone\n"))

		for _, one := range collection.Environments {
			fmt.Fprintf(stdout, "  %-28s was at %s\n", one.Name, one.Root)
		}

		fmt.Fprintf(stdout, "\n  %d container group(s), %d volume(s)\n",
			len(collection.Environments), len(collection.Volumes))
	}

	if len(collection.Worktrees) > 0 {
		fmt.Fprintf(stdout, "\n%s\n", header("Branch environments whose worktree is gone\n"))

		for _, one := range collection.Worktrees {
			fmt.Fprintf(stdout, "  %-28s was at %s\n", one.Name, one.Path)
		}
	}

	//
	// Listed and never removed from here: that file needs a password, other things depend on it,
	// and a command that quietly rewrites it in the middle of an unrelated cleanup is one nobody
	// trusts twice.
	//
	if len(collection.Hosts) > 0 {
		fmt.Fprintf(stdout, "\n%s\n", header("Entries in /etc/hosts with no environment left\n"))

		for _, domain := range collection.Hosts {
			fmt.Fprintf(stdout, "  %-32s %s set-host --remove %s\n", domain, binaryName(), domain)
		}

		fmt.Fprint(stdout,
			"\n  Not removed from here: that file needs a password and other things depend on it.\n")
	}

	if len(collection.Unattributable.Volumes) > 0 || len(collection.Unattributable.Environments) > 0 {
		fmt.Fprintf(stdout, "\n")
		fmt.Fprint(stdout, warning("Cannot be attributed, so they are left alone\n\n"))
		fmt.Fprint(stdout, "  Volumes carry no hm labels, so a project with no containers left could\n")
		fmt.Fprint(stdout, "  belong to anything. These are yours to judge:\n\n")

		for _, volume := range collection.Unattributable.Volumes {
			fmt.Fprintf(stdout, "  %s\n", volume)
		}

		for _, one := range collection.Unattributable.Environments {
			fmt.Fprintf(stdout, "  %-28s %s\n", one.Name, one.Reason)
		}
	}

	fmt.Fprintf(stdout, "\n")
}
