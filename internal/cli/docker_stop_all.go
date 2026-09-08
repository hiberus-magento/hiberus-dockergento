package cli

import (
	"io"
	"os"
)

// dockerStopAll stops every container running on this machine, machine-wide and with no label
// scoping — the same reach the shell implementation had, now asked of the engine directly rather
// than through a shell process.
//
// It resolves the project the way every other command does, but never refuses when the directory
// is not one: the shell implementation ran anywhere, and this still does.
func dockerStopAll(_ []string, stdout, stderr io.Writer, jsonOutput bool) int {
	engine := newEngine(stdout, stderr, jsonOutput)

	if _, err := engine.Resolve(here()); err != nil {
		return report(stderr, jsonOutput, "docker-stop-all", err)
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	result, err := engine.StopEverything(here(), interactive)
	if err != nil {
		return report(stderr, jsonOutput, "docker-stop-all", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "docker-stop-all", map[string]any{
			"total": result.Total, "others": result.Others, "stopped": result.Stopped > 0,
		})
	}

	return exitOK
}
