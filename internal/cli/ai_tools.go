package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
)

//
// ai-init, ai-pull and ai-reset, ported from console/commands/ai-{init,pull,reset}.sh. What they
// do is app.AITools; here are their options, their usage texts and the console they print on.
//
// None of them needs a project or a daemon: bin/run validated neither for them. And none answers
// with a document under --json — the scripts never did; their messages go to stderr instead, and
// the few lines they echoed still go to stdout.
//

// aiConsole is the terminal as the scripts' printers used it.
type aiConsole struct {
	stdout, stderr io.Writer
	json           bool
	command        string
}

var _ app.AIConsole = aiConsole{}

func (c aiConsole) colour(level string) string {
	switch level {
	case app.AIWarning:
		return "\033[0;33m"
	case app.AIError:
		return "\033[0;31m"
	case app.AIHeader:
		return "\033[1;37m"
	default:
		return "\033[0;32m"
	}
}

func (c aiConsole) Say(level, text string, how int) {
	switch {
	case how == app.AILine:
		fmt.Fprint(c.stderr, paint(c.colour(level), text)+"\n")
	case c.json:
		fmt.Fprint(c.stderr, text)
	case how == app.AIDecoratedErr:
		fmt.Fprint(c.stderr, paint(c.colour(level), text))
	default:
		fmt.Fprint(c.stdout, paint(c.colour(level), text))
	}
}

func (c aiConsole) Stdout(text string) { fmt.Fprint(c.stdout, text) }
func (c aiConsole) Stderr(text string) { fmt.Fprint(c.stderr, text) }
func (c aiConsole) JSON() bool         { return c.json }

// ReadLine is `read -r`: a line from standard input, read a byte at a time so that answers piped
// in for later questions are still there for them.
func (c aiConsole) ReadLine() string {
	line, _ := readLine(os.Stdin)

	return strings.Trim(line, " \t\n")
}

// Fail is hm_fail, which the wizard called from inside a command substitution: it ended that
// question and not the command, so this returns.
func (c aiConsole) Fail(status int, kind, message, hint string) {
	if c.json {
		document, err := json.MarshalIndent(contract.Failure(c.command, status, kind, message, hint), "", "  ")
		if err == nil {
			fmt.Fprintf(c.stderr, "%s\n", document)
		}

		return
	}

	fmt.Fprint(c.stderr, bad("\n"+message+"\n"))

	if hint != "" {
		fmt.Fprint(c.stderr, code(reset)+"\n  "+paint("\033[0;33m", hint)+code(reset)+"\n\n")
	}
}

func interactive() bool { return os.Getenv("HM_NON_INTERACTIVE") == "" }

const aiInitUsage = `Usage: hm ai-init [OPTIONS]

Initialize AI tools configuration via interactive wizard and download skills/agents.

Options:
  --platforms=<list>    Comma-separated AI platforms (claude,cursor,codex,copilot,gemini,opencode)
  --types=<list>        Comma-separated skill types (hyva,acs,magento,php)
  --resources=<list>    Resource types to download (skills,agents,both)
  --repository=<url>    Custom repository URL (HTTPS only)
  --branch=<name>       Branch name for custom repository (default: main)

Examples:
  hm ai-init
  hm ai-init --platforms=claude,cursor --types=hyva,magento --resources=skills,agents
  hm ai-init --repository=https://github.com/org/repo --branch=main

Interactive mode:
  Run without options for an interactive wizard that guides you through configuration.

Reconfiguration:
  Run on a project with existing ai-properties.json to modify the configuration.
  The wizard will pre-fill existing selections.

`

// aiInit configures the project's AI tooling and installs it for the first time.
//
// An option it does not know is refused the way the script refused it: the complaint with
// print_error, so with no newline after it and on stdout in text, then the usage, then 2.
func aiInit(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	console := aiConsole{stdout, stderr, jsonOutput, "ai-init"}
	options := app.AIInitOptions{Branch: "main"}

	for _, argument := range args {
		name, value, _ := strings.Cut(argument, "=")
		if !strings.Contains(argument, "=") {
			name = ""
		}

		switch name {
		case "--platforms":
			options.Platforms = value
		case "--types":
			options.Types = value
		case "--resources":
			options.Resources = value
		case "--repository":
			options.Repository = value
		case "--branch":
			options.Branch = value
		default:
			console.Say(app.AIError, "Unknown option: "+argument, app.AIDecorated)
			console.Stdout(aiInitUsage)

			return exitUsage
		}
	}

	return newEngine(stdout, stderr, jsonOutput).AIInit(here(), options, interactive(), console)
}
