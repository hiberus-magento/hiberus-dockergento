package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Branch environments.
//
// `list` and `remove` are here; `add` is still the shell implementation's. They read and write the
// same registrations, so there is no moment where the two disagree — what one writes the other
// sees, which is the only way a command can be ported one half at a time.
var worktreeSubcommands = map[string]bool{"list": true, "remove": true}

func worktree(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) == 0 {
		return exitOK
	}

	switch args[0] {
	case "list":
		return worktreeList(stdout, stderr, jsonOutput)
	case "remove":
		return worktreeRemove(args[1:], stdout, stderr, jsonOutput)
	}

	return exitOK
}

func worktreeList(stdout, stderr io.Writer, jsonOutput bool) int {
	if _, code := projectOr(stderr, jsonOutput, "worktree"); code != 0 {
		return code
	}

	project, listed, err := engine(stdout, stderr, jsonOutput).Worktrees(here())
	if err != nil {
		return report(stderr, jsonOutput, "worktree", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "worktree", map[string]any{
			"project": project, "worktrees": listed,
		})
	}

	if len(listed) == 0 {
		fmt.Fprint(stdout, good("No branch environments for this project.\n"))
		fmt.Fprintf(stdout, "  %s worktree add <branch>\n", binaryName())

		return exitOK
	}

	fmt.Fprintf(stdout, "\n%s\n", header("Branch environments of "+project+"\n"))

	missing := false

	for _, one := range listed {
		fmt.Fprintf(stdout, "  %-18s %-24s %-7s %-9s %s\n",
			one.Name, one.Branch, one.Profile, one.State, one.URL)

		if one.State == "missing" {
			missing = true
		}
	}

	fmt.Fprintf(stdout, "\n")

	//
	// `remove` is the tidy path and it needs the directory to still be there. When somebody has
	// already deleted it, what is left is a registration, and the command that collects those is
	// the one that collects everything else abandoned.
	//
	if missing {
		fmt.Fprint(stderr, warning("Some of these no longer have a worktree on disk\n"))
		fmt.Fprint(stdout, "  Collect them with ")
		fmt.Fprint(stdout, warning(binaryName()+" clean"))
		fmt.Fprintf(stdout, "\n\n")
	}

	return exitOK
}

func worktreeRemove(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	name := ""

	for _, argument := range args {
		if strings.HasPrefix(argument, "-") {
			return failure(stderr, jsonOutput, "worktree", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" worktree remove <name>")
		}

		name = argument
	}

	if _, code := projectOr(stderr, jsonOutput, "worktree"); code != 0 {
		return code
	}

	removed, err := engine(stdout, stderr, jsonOutput).RemoveWorktree(here(), name,
		os.Getenv("HM_FORCE") == "1", os.Getenv("HM_NON_INTERACTIVE") == "")
	if err != nil {
		return report(stderr, jsonOutput, "worktree", err)
	}

	// Nothing happened, because somebody said so
	if removed == "" {
		fmt.Fprint(stdout, good("Nothing was removed.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "worktree", map[string]any{"removed": removed})
	}

	fmt.Fprint(stdout, good("Removed "))
	fmt.Fprint(stdout, warning(removed))
	fmt.Fprintf(stdout, "\n")

	return exitOK
}
