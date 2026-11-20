package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// installation is which build of the tool is installed at root: a test answers it.
var installation = func(root string) core.Installation { return gitvcs.Git{}.Installed(root) }

// versionFlag answers `hm --version` and `hm -v`, ported from process_version in
// console/helpers/process_hm_options.sh.
//
// It is the shortest path in the CLI and the startup floor the performance budget watches, so it
// asks git about the installation and nothing else — Docker and Compose are the `version`
// subcommand's, which is the long answer. Its JSON is the shell's envelope with the fields in the
// shell's order, which is why it is not built from a map.
func versionFlag(stdout, stderr io.Writer, jsonOutput bool) int {
	installed := installation(toolRoot())

	if jsonOutput {
		var body bytesWriter

		encoder := json.NewEncoder(&body)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(contract.Success("version", installed)); err != nil {
			fmt.Fprintln(stderr, err)

			return exitError
		}

		io.WriteString(stdout, string(body)) //nolint:errcheck

		return exitOK
	}

	fmt.Fprint(stdout, paint("\033[1m", binaryName()+" "+installed.Version+"\n"))

	if installed.Detached {
		fmt.Fprintf(stdout, "  %-12s %s\n", "version", orUnknown(installed.Tag)+" (detached checkout)")
	} else {
		fmt.Fprintf(stdout, "  %-12s %s\n", "branch", installed.Branch)
	}

	fmt.Fprintf(stdout, "  %-12s %s\n", "commit", orUnknown(installed.Commit))
	fmt.Fprintf(stdout, "  %-12s %s\n", "installed", installed.Path)

	if installed.Dirty {
		fmt.Fprint(stdout, warning("  uncommitted changes in the installation directory\n"))
	}

	fmt.Fprint(stdout, "\n")

	return exitOK
}

// bytesWriter collects what an encoder writes.
type bytesWriter []byte

func (b *bytesWriter) Write(p []byte) (int, error) {
	*b = append(*b, p...)

	return len(p), nil
}
