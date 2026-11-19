package cli

import (
	"fmt"
	"io"
)

// compatibility prints the table of the Magento releases this tool has images for. It needs no
// project, takes no arguments, and prints the table under --json as well, as the shell did: there
// is no document to give, only the table to read.
func compatibility(stdout, stderr io.Writer, jsonOutput bool) int {
	table, err := newEngine(stdout, stderr, jsonOutput).Compatibility()
	if err != nil {
		return failure(stderr, jsonOutput, "compatibility", exitError, "data_unreadable", err.Error(), "")
	}

	fmt.Fprint(stdout, paint("\033[0;36m", table))

	return exitOK
}
