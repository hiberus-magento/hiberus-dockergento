package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// stableBranch is where `--stable` goes.
const stableBranch = "main"

// bold is a heading: structure, not content, so weight and no colour of its own.
func bold(text string) string { return paint("\033[1m", text) }

// switchVersion moves this tool's installation to a version or a branch, or lists what it can be
// moved to — with nothing to switch to, or with --list, which wins over a reference named with it.
//
// Ported from switch.sh: the same options, the same refusals, the same text. A reference named
// twice is the last one, as the shell's loop left it.
func switchVersion(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	target := ""
	listOnly := false

	for _, argument := range args {
		switch {
		case argument == "--list":
			listOnly = true
		case argument == "--stable":
			target = stableBranch
		case strings.HasPrefix(argument, "-"):
			return failure(stderr, jsonOutput, "switch", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" switch --help")
		default:
			target = argument
		}
	}

	tool := newEngine(stdout, stderr, jsonOutput)

	if listOnly || target == "" {
		versions, err := tool.Versions()
		if err != nil {
			return report(stderr, jsonOutput, "switch", err)
		}

		if jsonOutput {
			// Empty lists are lists: jq gave [] for nothing, and a program iterates over them
			if versions.Versions == nil {
				versions.Versions = []core.Version{}
			}
			if versions.Branches == nil {
				versions.Branches = []string{}
			}

			return document(stdout, stderr, "switch", versions)
		}

		printVersions(stdout, versions)

		return exitOK
	}

	switched, err := tool.Switch(target)

	var dirty core.DirtyInstallation
	if errors.As(err, &dirty) && !jsonOutput {
		// A person is shown what is in the way, on stdout as switch.sh printed it
		fmt.Fprint(stdout, bad("\nThe installation directory has uncommitted changes:\n\n"))
		for _, change := range strings.Split(dirty.Changes, "\n") {
			fmt.Fprintf(stdout, "    %s\n", change)
		}
		fmt.Fprint(stdout, code(reset)+"\nNothing was changed. Commit or stash them first in:\n  ")
		fmt.Fprint(stdout, warning(dirty.Directory+"\n\n"))

		return exitBlocked
	}

	if err != nil {
		return report(stderr, jsonOutput, "switch", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "switch", switched)
	}

	fmt.Fprint(stdout, good("\nNow running Hiberus Dockergento "+switched.Version+"\n"))
	fmt.Fprint(stdout, code(reset)+"  What changed: ")
	fmt.Fprint(stdout, warning("changelogs/\n"))
	fmt.Fprint(stdout, "\n")

	return exitOK
}

// printVersions is switch.sh's list: the versions, a pre-release labelled so it is not picked by
// mistake, the branches, and the installed one marked in either.
func printVersions(out io.Writer, versions core.Versions) {
	fmt.Fprint(out, "\n")
	fmt.Fprint(out, bold("Versions\n"))

	for _, version := range versions.Versions {
		switch {
		case version.Installed:
			fmt.Fprintf(out, "  %-22s", version.Name)
			fmt.Fprint(out, good("← installed"))
			if version.PreRelease {
				fmt.Fprint(out, "   pre-release")
			}
			fmt.Fprint(out, "\n")
		case version.PreRelease:
			fmt.Fprintf(out, "  %-22s", version.Name)
			fmt.Fprint(out, warning("pre-release\n"))
		default:
			fmt.Fprintf(out, "  %s\n", version.Name)
		}
	}

	fmt.Fprint(out, "\n")
	fmt.Fprint(out, bold("Branches\n"))

	for _, branch := range versions.Branches {
		if branch == versions.Current {
			fmt.Fprintf(out, "  %s", branch)
			fmt.Fprint(out, good("   ← installed\n"))
		} else {
			fmt.Fprintf(out, "  %s\n", branch)
		}
	}

	fmt.Fprint(out, "\n")
	fmt.Fprint(out, code(reset)+"Switch with ")
	fmt.Fprint(out, warning(binaryName()+" switch <version|branch>"))
	fmt.Fprint(out, code(reset)+", or back to stable with ")
	fmt.Fprint(out, warning(binaryName()+" switch --stable"))
	fmt.Fprint(out, code(reset)+"\n\n")
}
