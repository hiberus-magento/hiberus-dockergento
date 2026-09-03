package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The tool's answers and its exit codes live in the engine, because the terminal is not the only
// door: the HTTP adapter answers with the same documents, and so will MCP.
const (
	exitOK      = contract.ExitOK
	exitError   = contract.ExitError
	exitUsage   = contract.ExitUsage
	exitDocker  = contract.ExitDocker
	exitProject = contract.ExitProject
	exitService = contract.ExitService
	exitBlocked = contract.ExitBlocked
)

// asRefusal unwraps a refusal, which is the only error the layers below deliberately shape.
func asRefusal(err error, target *core.Refusal) bool {
	return errors.As(err, target)
}

// isTerminal reports whether there is somebody watching.
//
// Asked of the terminal itself and not of the file mode. `os.ModeCharDevice` is also true of
// /dev/null, so `hm describe > /dev/null` looked like a terminal and answered with a table where
// the shell implementation answers with JSON — and `hm exec` asked Docker for a pseudo-terminal
// that was not there, which is a hard failure rather than a wrong-looking one.
func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(file.Fd()))
}

// exitInterrupted is what aborting a question means, and it is the code a shell uses for it.
const exitInterrupted = 130

// refusal is an error shaped the way the layers below shape theirs, so that something the command
// line decided is reported exactly like something the engine decided.
func refusal(kind string, code int, message, hint string) error {
	return core.Refusal{Kind: kind, Code: code, Message: message, Hint: hint}
}

// failure reports an error the way the contract says: on stderr, structured when the output is
// JSON, and with the exit code that says which kind of failure it was.
func failure(stderr io.Writer, jsonOutput bool, command string, code int, kind, message, hint string) int {
	if jsonOutput {
		document, err := json.MarshalIndent(contract.Failure(command, code, kind, message, hint), "", "  ")
		if err == nil {
			fmt.Fprintf(stderr, "%s\n", document)

			return code
		}
	}

	fmt.Fprintf(stderr, "\n%s\n", message)
	if hint != "" {
		fmt.Fprintf(stderr, "  → %s\n\n", hint)
	}

	return code
}
