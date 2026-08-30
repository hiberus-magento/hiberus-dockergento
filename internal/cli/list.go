package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/dockerd"
	"github.com/hiberus-magento/hiberus-dockergento/internal/adapters/osfs"
	"github.com/hiberus-magento/hiberus-dockergento/internal/app"
	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

// list is the first command that stopped going through the shell implementation.
//
// It was chosen first for what it is: read-only, entirely about Docker, and with a JSON contract
// to compare against. Nothing it does can damage anything, and the comparison test says whether
// the two implementations agree — which is the only way to port the rest with any confidence.
func list(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) > 0 {
		return failure(stderr, jsonOutput, "list", exitUsage, "invalid_argument",
			fmt.Sprintf("Unknown option: %s", args[0]), "hm list --help")
	}

	inventory := app.Inventory{
		Engine:   dockerd.Engine{Timeout: 10 * time.Second},
		FS:       osfs.FS{},
		Branches: osfs.Branches{},
	}

	environments, err := inventory.Environments()
	if err != nil {
		return failure(stderr, jsonOutput, "list", exitDocker, "docker_unavailable",
			"Docker is not running", "Start Docker and try again")
	}

	if jsonOutput {
		return listAsJSON(environments, stdout, stderr)
	}

	listAsText(environments, stdout)

	return 0
}

func listAsJSON(environments []core.Environment, stdout, stderr io.Writer) int {
	document, err := json.MarshalIndent(envelope{
		SchemaVersion: 1,
		Command:       "list",
		OK:            true,
		Data: map[string]any{
			"environments": environments,
			"count":        len(environments),
		},
	}, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)

		return exitError
	}

	fmt.Fprintf(stdout, "%s\n", document)

	return 0
}

func listAsText(environments []core.Environment, stdout io.Writer) {
	if len(environments) == 0 {
		fmt.Fprintf(stdout, "\nNo Hiberus Dockergento environments found on this machine.\n\n")
		fmt.Fprintf(stdout, "  Create one with hm setup inside a Magento project.\n\n")

		return
	}

	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "   %-26s %-9s %-7s %-22s %s\n", "PROJECT", "STATUS", "SERVICE", "BRANCH", "ROOT")
	fmt.Fprintf(stdout, "   %-26s %-9s %-7s %-22s %s\n",
		"--------------------------", "---------", "-------", "----------------------", "----")

	for _, environment := range environments {
		name := environment.Name
		if environment.Worktree != "" {
			name = fmt.Sprintf("%s (wt: %s)", name, environment.Worktree)
		}

		marker := ""
		if environment.Orphan {
			marker = "  ⚠ orphan"
		}
		if !environment.HasMetadata {
			marker += "  (no metadata)"
		}

		fmt.Fprintf(stdout, "   %-26s %-9s %-7s %-22s %s%s\n",
			name,
			environment.Status,
			fmt.Sprintf("%d/%d", environment.Containers.Running, environment.Containers.Total),
			orDash(environment.Branch),
			orDash(environment.Root),
			marker)
	}

	fmt.Fprintf(stdout, "\n")
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}

	return value
}
