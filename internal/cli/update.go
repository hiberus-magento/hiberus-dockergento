package cli

import (
	"fmt"
	"io"
)

// update pulls this tool's installation and regenerates the shell completion.
//
// It takes no options: the shell implementation ignored whatever it was given, and an option that
// silently does nothing is one somebody believes did something.
func update(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 {
		return failure(stderr, jsonOutput, "update", exitUsage, "invalid_argument",
			fmt.Sprintf("Unknown option: %s", args[0]), binaryName()+" update")
	}

	if err := newEngine(stdout, stderr, jsonOutput).Update(); err != nil {
		return report(stderr, jsonOutput, "update", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "update", map[string]any{"updated": true})
	}

	fmt.Fprint(stdout, good("Hiberus Dockergento updated!\n"))

	return exitOK
}
