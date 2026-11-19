package cli

import (
	"fmt"
	"io"
)

// configEnv points the project's store at this machine: base URLs, cookie domain and the rest of
// config/docker/local_settings.json, one `bin/magento` command each.
//
// It takes no options. The shell implementation ignored whatever it was given; an option that
// silently does nothing is one somebody believes did something, so it is refused instead.
func configEnv(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 {
		return failure(stderr, jsonOutput, "config-env", exitUsage, "invalid_argument",
			fmt.Sprintf("Unknown option: %s", args[0]), binaryName()+" config-env")
	}

	if _, code := projectOr(stderr, jsonOutput, "config-env"); code != 0 {
		return code
	}

	if err := newEngine(stdout, stderr, jsonOutput).ConfigureForDevelopment(here()); err != nil {
		return report(stderr, jsonOutput, "config-env", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "config-env", map[string]any{"configured": true})
	}

	return exitOK
}
