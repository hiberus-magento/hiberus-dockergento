package cli

import (
	"io"
	"sort"
	"strings"
)

// completionMacOnly is what generate_completion.sh left out on Linux: the two commands of
// Composer's macOS vendor dance, which have nothing to copy anywhere else.
var completionMacOnly = map[string]bool{"copy-from-container": true, "copy-to-container": true}

// completion answers `hm completion bash|zsh`: the shell completion, printed for the shell to
// source. The commands are the keys of data/command_descriptions.json that do not start with "_",
// sorted the way jq's keys sorted them — the list generate_completion.sh writes into
// console/hm-completion.bash, so the bash script printed here is that file byte for byte.
func completion(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	usage := binaryName() + " completion <bash|zsh>"

	if len(args) != 1 || (args[0] != "bash" && args[0] != "zsh") {
		return failure(stderr, jsonOutput, "completion", exitUsage, "invalid_argument",
			"Name the shell to complete for: bash or zsh", usage)
	}

	commands, err := completionCommands()
	if err != nil {
		return failure(stderr, jsonOutput, "completion", exitError, "data_unreadable", err.Error(), "")
	}

	if args[0] == "bash" {
		io.WriteString(stdout, bashCompletion(commands)) //nolint:errcheck
	} else {
		io.WriteString(stdout, zshCompletion(commands)) //nolint:errcheck
	}

	return exitOK
}

// completionCommands is every command this machine can run, sorted.
func completionCommands() ([]string, error) {
	descriptions, err := readAssetObject("data/command_descriptions.json")
	if err != nil {
		return nil, err
	}

	mac := helpMachine() == "mac"
	commands := make([]string, 0, len(descriptions))

	for name := range descriptions {
		if strings.HasPrefix(name, "_") || (!mac && completionMacOnly[name]) {
			continue
		}

		commands = append(commands, name)
	}

	sort.Strings(commands)

	return commands, nil
}

// bashCompletion is what generate_completion.sh's `echo -e` wrote: every command on a line of its
// own inside the word list, the last one followed by a newline too.
func bashCompletion(commands []string) string {
	words := ""
	for _, command := range commands {
		words += command + "\n"
	}

	return "#!/usr/bin/env bash\n\ncomplete -W \"" + words + "\" " + binaryName() + "\n"
}

// zshCompletion is the same list for zsh's own completion system: offered for the first word,
// where a command goes. Sourced from .zshrc after compinit, or put on $fpath as _hm.
func zshCompletion(commands []string) string {
	name := binaryName()
	function := "_" + strings.NewReplacer("-", "_", ".", "_").Replace(name)

	return "#compdef " + name + "\n\n" +
		function + "() {\n" +
		"  if (( CURRENT == 2 )); then\n" +
		"    compadd -- " + strings.Join(commands, " ") + "\n" +
		"  fi\n" +
		"}\n\n" +
		"if [ \"$funcstack[1]\" = \"" + function + "\" ]; then\n" +
		"  " + function + " \"$@\"\n" +
		"else\n" +
		"  compdef " + function + " " + name + "\n" +
		"fi\n"
}
