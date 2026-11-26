package cli

import (
	"fmt"
	"io"
)

//
// --dry-run: what a destroying command would remove, and nothing removed.
//
// Every preview answers --json with the same shape, whichever command it is, so that a program
// asks "what would go" once and reads the answer the same way for each:
//
//	{"dry_run": true, "project": "...", "would_remove": {"containers": [], "volumes": [], "networks": [],
//	 "databases": [], "files": []}}
//
// Every list is always there, empty when the command touches nothing of that kind. `clean` is the
// exception: it already previewed without --force, and its --dry-run is that preview, document
// and all.
//

// removal is what a command would remove, by kind.
type removal struct {
	containers, volumes, networks, databases, files []string
}

func dryRunDocument(stdout, stderr io.Writer, command, project string, removed removal,
	extra map[string]any) int {
	data := map[string]any{
		"dry_run": true,
		"project": project,
		"would_remove": map[string][]string{
			"containers": orEmpty(removed.containers),
			"volumes":    orEmpty(removed.volumes),
			"networks":   orEmpty(removed.networks),
			"databases":  orEmpty(removed.databases),
			"files":      orEmpty(removed.files),
		},
	}

	for key, value := range extra {
		data[key] = value
	}

	return document(stdout, stderr, command, data)
}

// print is the text list: one line per thing, its kind first.
func (r removal) print(stdout io.Writer) {
	for _, kind := range []struct {
		name   string
		things []string
	}{
		{"container", r.containers}, {"volume", r.volumes}, {"network", r.networks},
		{"database", r.databases}, {"file", r.files},
	} {
		for _, thing := range kind.things {
			fmt.Fprintf(stdout, "  %-10s %s\n", kind.name, thing)
		}
	}
}

func (r removal) empty() bool {
	return len(r.containers)+len(r.volumes)+len(r.networks)+len(r.databases)+len(r.files) == 0
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}
