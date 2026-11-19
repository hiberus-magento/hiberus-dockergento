package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// aiDoctor says what AI tooling this project has, where each piece came from, and whether it is
// still that. It changes nothing.
//
// Ported from console/commands/ai-doctor.sh, whose output this is line for line. Only the first
// argument is looked at, as the script's `case "${1:-}"` looked at it; --help never reaches here,
// because the router answers it with the command's help as bin/run did. It needs no project and
// no daemon: bin/run validated neither for it.
func aiDoctor(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 && args[0] != "" {
		return failure(stderr, jsonOutput, "ai-doctor", exitUsage, "invalid_argument",
			fmt.Sprintf("Unknown option: %s", args[0]), binaryName()+" ai-doctor")
	}

	report, err := newEngine(stdout, stderr, jsonOutput).AIDoctor(here())
	if errors.Is(err, core.ErrAIRegistrationCorrupt) {
		// The script read the registration inside a command substitution: in text its complaint
		// went into the variable and nowhere else, and under --json print_error and print_warning
		// wrote it to stderr with no newline between or after them
		if jsonOutput {
			fmt.Fprint(stderr, "Invalid JSON in "+core.AIRegistrationFile+
				"Registration file is corrupted - refusing to operate")
		}

		return exitProject
	}

	if err != nil {
		return failure(stderr, jsonOutput, "ai-doctor", exitError, "read_failed", err.Error(), "")
	}

	if jsonOutput {
		return document(stdout, stderr, "ai-doctor", report)
	}

	if !report.Configured {
		fmt.Fprint(stdout, good("This project has no AI tooling configured.\n"))
		fmt.Fprint(stdout, code(reset)+"  "+binaryName()+" ai-init\n")

		return exitOK
	}

	if len(report.Resources) == 0 {
		fmt.Fprint(stdout, good("Nothing installed yet.\n"))
		fmt.Fprint(stdout, code(reset)+"  "+binaryName()+" ai-pull\n")

		return exitOK
	}

	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, paint("\033[1m", "AI tooling in this project\n\n"))
	fmt.Fprintf(stdout, "  %-10s %-30s %-9s %-24s %s\n", "PLATFORM", "NAME", "STATE", "FROM", "INSTALLED")

	outdated, modified := 0, 0

	for _, resource := range report.Resources {
		from := resource.Origin
		if resource.Version != "-" {
			from += " " + resource.Version
		}

		fmt.Fprintf(stdout, "  %-10s %-30s %-9s %-24s %s\n",
			resource.Platform, resource.Name, resource.State, from, resource.Installed)

		// The script counted with grep -c over its rows, so the word anywhere in a row counts it
		row := strings.Join([]string{resource.Platform, resource.Kind, resource.Name, resource.State,
			resource.Origin, resource.Version, resource.Installed}, "\t")

		if strings.Contains(row, core.AIStateOutdated) {
			outdated++
		}

		if strings.Contains(row, core.AIStateModified) {
			modified++
		}
	}

	fmt.Fprint(stdout, "\n")

	if outdated > 0 {
		fmt.Fprint(stderr, warning(fmt.Sprintf("%d behind the version that came with this tool", outdated))+"\n")
		fmt.Fprint(stdout, code(reset)+"  "+binaryName()+" ai-pull\n")
	}

	if modified > 0 {
		fmt.Fprint(stderr, warning(fmt.Sprintf("%d changed since they were installed", modified))+"\n")
		fmt.Fprint(stdout, code(reset)+"  Those edits are lost on the next "+binaryName()+" ai-pull.\n")
		fmt.Fprint(stdout, code(reset)+"  Rename them to keep them: what the tool did not install, it does not touch.\n")
	}

	fmt.Fprint(stdout, "\n")

	return exitOK
}
