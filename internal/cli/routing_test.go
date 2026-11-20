package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strconv"
	"strings"
	"testing"
)

//
// What the router does with the docker tools, as opposed to what their handlers ask. A command
// that is not a case in run.go's switch falls through to the project-command lookup, which
// answering(t) pins to a fresh temporary directory with no project in it — so an unrouted call is
// refused as not found and records nothing.
//

func TestTheRouterAnswersForTheDockerTools(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"docker-stop-all", []string{"docker-stop-all", "--yes"}},
		{"docker-compose", []string{"docker-compose", "config"}},
		{"debug-off", []string{"debug-off"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			Run(tt.args, io.Discard, io.Discard)

			//
			// The fallthrough (a project's own command) never reaches newEngine — so as long as the
			// command falls through unrouted, the fake this test
			// substituted for newEngine is asked nothing at all. A case line in the switch is what
			// makes the handler reach the fake instead.
			//
			if len(fake.calls) == 0 {
				t.Fatalf("%v asked the fake engine nothing, want the router to reach the handler "+
					"instead of falling through to the project-command lookup", tt.args)
			}
		})
	}
}

//
// `hm <command> --help` is the command's help for every command, before any handler runs: a
// handler that took it as an argument answered "unknown subcommand" or "not a project" instead.
// Every case of run.go's switch is read from the source, so a command ported later is held to it
// without being listed. The page itself is the one bin/run printed, from the goldens.
//

func TestHelpAfterAnyRoutedCommandIsTheShellsHelp(t *testing.T) {
	for _, command := range routedCommands(t) {
		for _, flag := range []string{"--help", "-h"} {
			t.Run(command+" "+flag, func(t *testing.T) {
				fake := answering(t)
				pinHelp(t, helpTool, "mac")
				t.Setenv("NO_COLOR", "1")

				var stdout, stderr strings.Builder

				code := Run([]string{command, flag}, &stdout, &stderr)

				if code != 0 || len(fake.calls) != 0 || stderr.String() != "" {
					t.Fatalf("%s %s answered %d with %q and %d engine calls, want its help and no "+
						"handler run", command, flag, code, stderr.String(), len(fake.calls))
				}

				if want := golden(t, "commands/"+command+".txt"); stdout.String() != want {
					t.Fatalf("%s %s = %q, want bin/run's %q", command, flag, stdout.String(), want)
				}
			})
		}
	}
}

// routedCommands are the string cases of Run's switch.
func routedCommands(t *testing.T) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "run.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var commands []string

	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}

		for _, expression := range clause.List {
			literal, ok := expression.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}

			name, _ := strconv.Unquote(literal.Value)
			if !strings.HasPrefix(name, "_") {
				commands = append(commands, name)
			}
		}

		return true
	})

	if len(commands) < 40 {
		t.Fatalf("read %d routed commands from run.go, want the whole switch", len(commands))
	}

	return commands
}
