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
// Both are here. They share nothing but the word: one writes a dump that outlives the environment
// and can be read by a database two versions later, the other replaces the files underneath a
// running one. What they do share is the reason to exist — a copy taken before something risky —
// and that is why they are one command.
//

// templateSubcommands are the ones handled here, which is now both families.
var templateSubcommands = map[string]bool{
	"freeze": true, "templates": true, "clone": true, "drop": true,
	"snapshot": true, "list": true, "restore": true, "remove": true, "clear": true,
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
	case "snapshot":
		return snapshot(rest, stdout, stderr, jsonOutput, forced)
	case "list":
		return snapshots(stdout, stderr, jsonOutput)
	case "restore":
		return restore(rest, stdout, stderr, jsonOutput)
	case "remove":
		return removeSnapshot(rest, stdout, stderr, jsonOutput)
	case "clear":
		return clearSnapshots(rest, stdout, stderr, jsonOutput)
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

//
// The snapshots: a named copy of the database, as a dump in a file.
//

func snapshot(args []string, stdout, stderr io.Writer, jsonOutput, forced bool) int {
	name := ""

	for at := 0; at < len(args); at++ {
		argument := args[at]

		switch {
		case strings.HasPrefix(argument, "--name="):
			name = strings.TrimPrefix(argument, "--name=")
		case argument == "--name":
			if at+1 < len(args) {
				at++
				name = args[at]
			}
		default:
			return failure(stderr, jsonOutput, "db", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" db snapshot --name=before-upgrade")
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	taken, err := engine(stdout, stderr, jsonOutput).TakeSnapshot(here(), name, forced)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"name": taken.Name, "path": taken.Path,
			"size": taken.Size, "taken_at": taken.TakenAt,
		})
	}

	fmt.Fprint(stdout, good("Saved "))
	fmt.Fprint(stdout, warning(taken.Name))
	fmt.Fprint(stdout, good(fmt.Sprintf(" (%s)\n", taken.Size)))

	return exitOK
}

func snapshots(stdout, stderr io.Writer, jsonOutput bool) int {
	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	project, found, err := engine(stdout, stderr, jsonOutput).Snapshots(here())
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if jsonOutput {
		listed := make([]map[string]any, 0, len(found))
		for _, one := range found {
			listed = append(listed, map[string]any{
				"name": one.Name, "taken_at": one.TakenAt, "size": one.Size,
			})
		}

		return document(stdout, stderr, "db", map[string]any{
			"project": project.Name, "snapshots": listed,
		})
	}

	if len(found) == 0 {
		fmt.Fprint(stdout, good("No snapshots for this project yet.\n"))
		fmt.Fprintf(stdout, "  %s db snapshot --name=before-upgrade\n", binaryName())

		return exitOK
	}

	fmt.Fprintf(stdout, "\n%s\n", header("Snapshots of "+project.Name+"\n"))

	for _, one := range found {
		fmt.Fprintf(stdout, "  %-28s %-18s %s\n", one.Name, one.TakenAt, one.Size)
	}

	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func restore(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	project, done, err := engine(stdout, stderr, jsonOutput).RestoreSnapshot(here(), name, interactive)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	// Nothing happened, because somebody said so
	if !done {
		fmt.Fprint(stdout, good("Nothing was restored.\n"))

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"restored": name, "project": project.Name,
		})
	}

	fmt.Fprint(stdout, good("Restored. Flush the cache with "))
	fmt.Fprint(stdout, warning(binaryName()+" magento cache:flush"))
	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func removeSnapshot(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	if err := engine(stdout, stderr, jsonOutput).RemoveSnapshot(here(), name); err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{"removed": name})
	}

	fmt.Fprint(stdout, good("Removed "))
	fmt.Fprint(stdout, warning(name))
	fmt.Fprintf(stdout, "\n")

	return exitOK
}

func clearSnapshots(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	all := false

	for _, argument := range args {
		if argument != "--all" {
			return failure(stderr, jsonOutput, "db", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" db clear [--all]")
		}

		all = true
	}

	if _, code := projectOr(stderr, jsonOutput, "db"); code != 0 {
		return code
	}

	interactive := os.Getenv("HM_NON_INTERACTIVE") == ""

	cleared, err := engine(stdout, stderr, jsonOutput).ClearSnapshots(here(), all, interactive)
	if err != nil {
		return report(stderr, jsonOutput, "db", err)
	}

	//
	// Three endings that are not the same: nothing to delete, somebody said no, and it happened.
	// The first two delete nothing, and only the last one is worth a number.
	//
	if cleared.Removed == 0 {
		if jsonOutput {
			return document(stdout, stderr, "db", map[string]any{"removed": 0, "freed": "0B"})
		}

		if cleared.Found > 0 {
			fmt.Fprint(stdout, good("Nothing was deleted.\n"))
		} else {
			fmt.Fprint(stdout, good("There are no snapshots to clear.\n"))
		}

		return exitOK
	}

	if jsonOutput {
		return document(stdout, stderr, "db", map[string]any{
			"removed": cleared.Removed, "freed": cleared.Freed, "scope": cleared.Scope,
		})
	}

	fmt.Fprint(stdout, good(fmt.Sprintf("Deleted %d snapshot(s), freeing %s.\n",
		cleared.Removed, cleared.Freed)))

	return exitOK
}
