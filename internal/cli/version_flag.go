package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/gitvcs"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// installation is which source checkout of the tool is installed at root: a test answers it.
var installation = func(root string) core.Installation { return gitvcs.Git{}.Installed(root) }

// installedBuild is this binary: the release it was stamped as, or, for a source build with no
// stamp, what git says about the checkout it was built in.
func installedBuild() core.Installation {
	if Version != "" {
		return core.ReleaseInstallation(Version, executablePath())
	}

	return installation(toolRoot())
}

// executablePath is the running binary with every symlink resolved, or empty.
func executablePath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		return resolved
	}

	return executable
}

// printInstallation is the part of `--version` and `version` that names the build: the branch
// and commit of a checkout, or nothing but the path for a release, which has neither.
func printInstallation(stdout io.Writer, installed core.Installation) {
	if installed.Release {
		fmt.Fprintf(stdout, "  %-12s %s\n", "installed", installed.Path)

		return
	}

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
}

// versionFlag answers `hm --version` and `hm -v`, ported from process_version in
// console/helpers/process_hm_options.sh.
//
// It is the shortest path in the CLI and the startup floor the performance budget watches, so it
// asks git about the installation and nothing else — Docker and Compose are the `version`
// subcommand's, which is the long answer. Its JSON is the shell's envelope with the fields in the
// shell's order, which is why it is not built from a map.
func versionFlag(stdout, stderr io.Writer, jsonOutput bool) int {
	installed := installedBuild()

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
	printInstallation(stdout, installed)
	fmt.Fprint(stdout, "\n")

	return exitOK
}

// bytesWriter collects what an encoder writes.
type bytesWriter []byte

func (b *bytesWriter) Write(p []byte) (int, error) {
	*b = append(*b, p...)

	return len(p), nil
}
