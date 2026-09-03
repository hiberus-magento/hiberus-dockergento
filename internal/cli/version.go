package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

//
// Which build of the tool this is, and what is underneath it.
//
// `hm --version` answers the first half and stops there, on purpose: it is the shortest path in
// the CLI, it has a performance budget watched by a test, and it must not need Docker. This
// command is the other half, which is what a bug report needs.
//

func version(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	for _, argument := range args {
		return failure(stderr, jsonOutput, "version", exitUsage, "invalid_argument",
			"Unknown option: "+argument, binaryName()+" version")
	}

	installed, tooling := engine(stdout, stderr, jsonOutput).Installed()

	if jsonOutput {
		return document(stdout, stderr, "version", map[string]any{
			"version": installed.Version, "tag": installed.Tag,
			"commits_ahead": installed.CommitsAhead, "commit": installed.Commit,
			"branch": installed.Branch, "detached": installed.Detached,
			"dirty": installed.Dirty, "path": installed.Path,
			"binary": buildOfThisBinary(),
			"docker": map[string]any{
				"version": tooling.Docker, "compose": tooling.Compose,
				"compose_command": tooling.ComposeCommand,
			},
		})
	}

	fmt.Fprint(stdout, header(binaryName()+" "+installed.Version+"\n"))

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

	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  %-12s %s\n", "docker", orMissing(tooling.Docker))
	fmt.Fprintf(stdout, "  %-12s %s\n", "compose", orMissing(tooling.Compose))
	//
	// Which build of the binary is running, which the shell implementation cannot answer about
	// itself. It is the last line rather than the first because it is the one nobody needs until
	// they are reporting something about the ported half.
	//
	fmt.Fprintf(stdout, "  %-12s %s\n", "binary", buildOfThisBinary())

	fmt.Fprintf(stdout, "\n")
	fmt.Fprintf(stdout, "  %s switch --list   to see the versions available\n", binaryName())
	fmt.Fprintf(stdout, "\n")

	return exitOK
}

// buildOfThisBinary is which build is running, which the shell implementation cannot answer about
// itself and which a bug report about the ported half needs.
func buildOfThisBinary() string {
	build := Version
	if build == "" {
		build = "dev"
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
				return build + " (" + setting.Value[:7] + ")"
			}
		}
	}

	return build
}

func orMissing(value string) string {
	if value == "" {
		return "not available"
	}

	return value
}
