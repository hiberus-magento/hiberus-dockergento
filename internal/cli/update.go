package cli

import (
	"fmt"
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// runningVersion is the release this binary was built as: the version GoReleaser stamps into
// Version. Go's own build information is not asked: the tags have no "v", so a module build never
// carries a release version, only "(devel)" or a pseudo-version. Empty is a binary built from a
// checkout.
func runningVersion() string { return Version }

// update replaces this binary with the newest release when there is a newer one. --pre lets that
// be a pre-release.
func update(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	// A JSON answer is read by a program, with nobody to give sudo a password: a binary in a
	// directory only root writes is then refused instead of moved to ~/.hm/bin
	options := core.UpdateOptions{Running: runningVersion(), Unattended: jsonOutput}

	for _, argument := range args {
		if argument != "--pre" {
			return failure(stderr, jsonOutput, "update", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), binaryName()+" update [--pre]")
		}

		options.PreReleases = true
	}

	updated, err := newEngine(stdout, stderr, jsonOutput).Update(options)
	if err != nil {
		return report(stderr, jsonOutput, "update", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "update", updated)
	}

	if !updated.Updated {
		fmt.Fprint(stdout, good("Hiberus Dockergento "+updated.From+" is up to date\n"))

		if updated.Latest != "" && updated.Latest != updated.From {
			fmt.Fprintf(stdout, "  The newest release is %s\n", updated.Latest)
		}

		return exitOK
	}

	fmt.Fprint(stdout, good("Hiberus Dockergento updated from "+updated.From+" to "+updated.To+"\n"))
	fmt.Fprintf(stdout, "  %-12s %s\n", "installed", updated.Path)
	fmt.Fprintf(stdout, "  %-12s %s\n", "what changed", releaseNotes(updated.To))

	return exitOK
}
