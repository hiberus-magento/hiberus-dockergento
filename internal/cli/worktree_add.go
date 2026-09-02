package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
)

// worktreeAdd gives a branch an environment of its own.
func worktreeAdd(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	options := app.AddOptions{Profile: "agent", Start: true, Anonymise: true}

	for at := 0; at < len(args); at++ {
		argument := args[at]

		value := func(prefix string) (string, bool) {
			if strings.HasPrefix(argument, prefix+"=") {
				return strings.TrimPrefix(argument, prefix+"="), true
			}

			if argument == prefix && at+1 < len(args) {
				at++

				return args[at], true
			}

			return "", false
		}

		if profile, ok := value("--profile"); ok {
			options.Profile = profile

			continue
		}

		if path, ok := value("--path"); ok {
			options.Path = path

			continue
		}

		switch argument {
		case "--no-start":
			options.Start = false
		case "--no-anonymise", "--no-anonymize":
			options.Anonymise = false
		default:
			if strings.HasPrefix(argument, "-") {
				return failure(stderr, jsonOutput, "worktree", exitUsage, "invalid_argument",
					"Unknown option: "+argument,
					binaryName()+" worktree add <branch> --profile=agent")
			}

			if options.Branch != "" {
				return failure(stderr, jsonOutput, "worktree", exitUsage, "too_many_arguments",
					"One branch at a time", binaryName()+" worktree add <branch>")
			}

			options.Branch = argument
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "worktree"); code != 0 {
		return code
	}

	added, err := engine(stdout, stderr, jsonOutput).AddWorktree(here(), options)
	if err != nil {
		return report(stderr, jsonOutput, "worktree", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "worktree", added)
	}

	fmt.Fprintf(stdout, "\n")
	fmt.Fprint(stdout, good("Branch environment "))
	fmt.Fprint(stdout, warning(added.Name))
	fmt.Fprint(stdout, good(" is at "))
	fmt.Fprint(stdout, warning(added.URL))
	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  Code:    %s\n", added.Path)
	fmt.Fprintf(stdout, "  Profile: %s\n", added.Profile)
	fmt.Fprintf(stdout, "  Run %s commands from that directory and they act on this environment.\n\n",
		binaryName())

	return exitOK
}
