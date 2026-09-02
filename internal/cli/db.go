package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
)

//
// `db` is two families that share a name: snapshots, which are dumps in a file, and templates,
// which are byte copies of a data directory in a volume. One is for keeping and the other for
// standing environments up.
//
// The templates are here. The snapshots are still the shell implementation's, and the router
// hands them over by subcommand — which is a boundary between two independent operations rather
// than a command split down the middle.
//

// templateSubcommands are the ones handled here.
var templateSubcommands = map[string]bool{
	"freeze": true, "templates": true, "clone": true, "drop": true,
}

func db(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) == 0 {
		return exitOK
	}

	subcommand, rest := args[0], args[1:]
	forced := os.Getenv("HM_FORCE") == "1"

	switch subcommand {
	case "templates":
		return templates(stdout, stderr, jsonOutput)
	case "freeze":
		return freeze(rest, stdout, stderr, jsonOutput, forced)
	case "clone":
		return clone(rest, stdout, stderr, jsonOutput, forced)
	case "drop":
		return drop(rest, stdout, stderr, jsonOutput, forced)
	}

	return exitOK
}

func templates(stdout, stderr io.Writer, jsonOutput bool) int {
	found, err := engine(stdout, stderr, jsonOutput).Templates()
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{"templates": found})
	}

	if len(found) == 0 {
		fmt.Fprint(stdout, good("No templates on this machine yet.\n"))
		fmt.Fprintf(stdout, "  %s db freeze --name=base\n", binaryName())

		return exitOK
	}

	fmt.Fprintf(stdout, "\n%s\n", header("Database templates\n"))

	for _, template := range found {
		fmt.Fprintf(stdout, "  %-28s %-9s %-28s %s\n",
			template.Address, template.Size, template.Image, template.Created)
	}

	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func freeze(args []string, stdout, stderr io.Writer, jsonOutput, forced bool) int {
	options := app.FreezeOptions{Force: forced}

	for at := 0; at < len(args); at++ {
		argument := args[at]

		switch {
		case strings.HasPrefix(argument, "--name="):
			options.Name = strings.TrimPrefix(argument, "--name=")
		case argument == "--name":
			if at+1 < len(args) {
				at++
				options.Name = args[at]
			}
		default:
			return failure(stderr, jsonOutput, "db", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" db freeze --name=base")
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	frozen, err := engine(stdout, stderr, jsonOutput).Freeze(here(), options)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"template": frozen.Address, "volume": frozen.Volume,
			"image": frozen.Image, "size": frozen.Size, "bytes": frozen.Bytes,
		})
	}

	fmt.Fprint(stdout, good("Frozen as "))
	fmt.Fprint(stdout, warning(frozen.Address))
	fmt.Fprint(stdout, good(fmt.Sprintf(" (%s). Build an environment from it with ", frozen.Size)))
	fmt.Fprint(stdout, warning(binaryName()+" db clone "+frozen.Address))
	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func clone(args []string, stdout, stderr io.Writer, jsonOutput, forced bool) int {
	address := ""
	if len(args) > 0 {
		address = args[0]
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	cloned, err := engine(stdout, stderr, jsonOutput).Clone(here(), address, forced)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	// Nothing happened, because somebody said so
	if cloned.Address == "" {
		fmt.Fprint(stdout, good("Nothing was cloned.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"cloned": cloned.Address, "project": cloned.Project, "volume": cloned.Volume,
		})
	}

	fmt.Fprint(stdout, good("Cloned. Start the environment with "))
	fmt.Fprint(stdout, warning(binaryName()+" start"))
	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func drop(args []string, stdout, stderr io.Writer, jsonOutput, forced bool) int {
	address := ""
	if len(args) > 0 {
		address = args[0]
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	dropped, err := engine(stdout, stderr, jsonOutput).Drop(here(), address, forced, interactive)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if dropped.Address == "" {
		fmt.Fprint(stdout, good("Nothing was deleted.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"dropped": dropped.Address, "freed": orZero(dropped.Size),
		})
	}

	fmt.Fprint(stdout, good(fmt.Sprintf("Dropped %s, freeing %s.\n", dropped.Address, orNothing(dropped.Size))))

	return exitOK
}

func orZero(size string) string {
	if size == "" {
		return "0"
	}

	return size
}

func orNothing(size string) string {
	if size == "" {
		return "nothing"
	}

	return size
}
