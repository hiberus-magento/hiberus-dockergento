package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// permissions prints the permissions an agent using this tool should be given, for its settings
// file. Nothing is written: the configuration file is the person's, and may have rules that matter
// to them.
//
// Under --json the configuration itself is printed, not wrapped in the contract's envelope: it is
// what gets pasted into the agent's settings, and that is how the shell implementation printed it.
func permissions(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	strict := false

	for _, argument := range args {
		if argument != "--strict" {
			return failure(stderr, jsonOutput, "permissions", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), binaryName()+" permissions [--strict]")
		}

		strict = true
	}

	configuration, err := newEngine(stdout, stderr, jsonOutput).AgentPermissions(strict)
	if err != nil {
		return report(stderr, jsonOutput, "permissions", err)
	}

	body, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return report(stderr, jsonOutput, "permissions", err)
	}

	if jsonOutput {
		fmt.Fprintf(stdout, "%s\n", body)

		return exitOK
	}

	allowed, asked, denied := len(configuration.Permissions.Allow), len(configuration.Permissions.Ask),
		len(configuration.Permissions.Deny)

	fmt.Fprintf(stdout, "\n%s\n\n", header("Permissions for an agent using "+binaryName()))

	if strict {
		fmt.Fprint(stdout, "  Strict: only commands with no side effects are allowed.\n\n")
	} else {
		fmt.Fprintf(stdout, "  %d command(s) allowed, %d asking for confirmation.\n\n", allowed, asked)
	}

	fmt.Fprintf(stdout, "  %d path(s) denied outright: secrets, customer data, and what is\n", denied)
	fmt.Fprint(stdout, "  generated rather than written. See docs/ai-context.md.\n\n")
	fmt.Fprintf(stdout, "%s\n\n", body)
	fmt.Fprint(stdout, "  Copy this into your agent's settings. Nothing was written for you: your\n")
	fmt.Fprint(stdout, "  configuration file is yours, and may have rules that matter to you.\n\n")

	return exitOK
}
