package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// bold is a heading: structure, not content, so weight and no colour of its own.
func bold(text string) string { return paint("\033[1m", text) }

// releaseNotes is where the changes of a version are read.
var releaseNotes = dockergento.ReleaseNotes

// switchVersion installs another release in place of this binary, or lists the releases — with
// nothing to switch to, or with --list, which wins over a version named with it. A version named
// twice is the last one.
func switchVersion(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options := core.SwitchOptions{Running: runningVersion()}
	listOnly := false

	for _, argument := range args {
		switch {
		case argument == "--list":
			listOnly = true
		case argument == "--stable":
			options.Stable, options.Version = true, ""
		case strings.HasPrefix(argument, "-"):
			return failure(stderr, jsonOutput, "switch", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" switch --help")
		default:
			options.Stable, options.Version = false, argument
		}
	}

	tool := newEngine(stdout, stderr, jsonOutput)

	if listOnly || (options.Version == "" && !options.Stable) {
		versions, err := tool.Versions(options.Running)
		if err != nil {
			return report(stderr, jsonOutput, "switch", err)
		}

		if jsonOutput {
			// Empty lists are lists: a program iterates over them
			if versions.Versions == nil {
				versions.Versions = []core.Version{}
			}

			return document(stdout, stderr, "switch", versions)
		}

		printVersions(stdout, versions)

		return exitOK
	}

	defer promptless(jsonOutput)()

	switched, err := tool.Switch(options)
	if err != nil {
		return report(stderr, jsonOutput, "switch", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "switch", switched)
	}

	if switched.Unchanged {
		fmt.Fprint(stdout, good("\nAlready running Hiberus Dockergento "+switched.Version+"\n\n"))

		return exitOK
	}

	fmt.Fprint(stdout, good("\nNow running Hiberus Dockergento "+switched.Version+"\n"))
	fmt.Fprint(stdout, code(reset)+"  What changed: ")
	fmt.Fprint(stdout, warning(releaseNotes(switched.Version)+"\n"))
	fmt.Fprint(stdout, "\n")

	return exitOK
}

// printVersions lists the releases, a pre-release labelled so it is not picked by mistake, and the
// running one marked.
func printVersions(out io.Writer, versions core.Versions) {
	fmt.Fprint(out, "\n")
	fmt.Fprint(out, bold("Versions\n"))

	if len(versions.Versions) == 0 {
		fmt.Fprint(out, "  no release of 2.0 or later is published yet\n")
	}

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
	fmt.Fprintf(out, "Running %s\n\n", versions.Current)
	fmt.Fprint(out, code(reset)+"Switch with ")
	fmt.Fprint(out, warning(binaryName()+" switch <version>"))
	fmt.Fprint(out, code(reset)+", or to the newest final release with ")
	fmt.Fprint(out, warning(binaryName()+" switch --stable"))
	fmt.Fprint(out, code(reset)+"\n\n")
}
