package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The exit codes are a contract that callers branch on, and they are the shell implementation's:
// a command ported to Go that answered differently would break scripts, agents and the tests that
// check them.
const (
	exitOK      = 0
	exitError   = 1
	exitUsage   = 2
	exitDocker  = 3
	exitProject = 4
	exitService = 5
	exitBlocked = 6
)

// envelope is the shape every command that reports something answers with. One schema, one
// version, so a reader can tell a tool that changed from a tool that broke.
type envelope struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	OK            bool   `json:"ok"`
	Data          any    `json:"data,omitempty"`
	Error         *fault `json:"error,omitempty"`
}

type fault struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// wantsJSON decides the output format the way the shell implementation does: what was asked for
// wins, and when nothing was asked, a terminal gets text and anything else gets JSON.
//
// The default matters more than it looks. A command whose output is piped is being read by a
// program, and a program reading a table of dashes is a program that breaks the first time a
// column widens.
func wantsJSON(args []string, stdout io.Writer) (bool, []string) {
	remaining := make([]string, 0, len(args))
	decided := false
	value := false

	for _, argument := range args {
		switch argument {
		case "--json":
			decided, value = true, true
		case "--no-json":
			decided, value = true, false
		default:
			remaining = append(remaining, argument)
		}
	}

	if decided {
		return value, remaining
	}

	return !isTerminal(stdout), remaining
}

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

// failure reports an error the way the contract says: on stderr, structured when the output is
// JSON, and with the exit code that says which kind of failure it was.
func failure(stderr io.Writer, jsonOutput bool, command string, code int, kind, message, hint string) int {
	if jsonOutput {
		document, err := json.MarshalIndent(envelope{
			SchemaVersion: 1,
			Command:       command,
			OK:            false,
			Error:         &fault{Code: code, Type: kind, Message: message, Hint: hint},
		}, "", "  ")
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
