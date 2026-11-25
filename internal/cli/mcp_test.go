package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// The MCP server, ported from console/commands/mcp.sh and console/tasks/mcp_tools.sh.
//
// The goldens in testdata/mcp are the shell script's own answers, produced before the port by
// piping the request files into it, with no Docker needed because none of those requests runs a
// command:
//
//	DOCKER_HOST=unix:///nonexistent bin/run mcp         < read.jsonl  > read.golden
//	DOCKER_HOST=unix:///nonexistent bin/run mcp --write < write.jsonl > write.golden
//
// Two kinds of line have moved on from the shell's since: the tools/list answer, whose catalogue
// gained the `environment` argument, the output schemas and the tools added after the port
// (mcp_protocol_test.go has what the schemas do), and the answer to an unknown
// tool, which lists the tools there are.
//
// The tool calls that do run a command are proved against a recorder standing in for this
// binary, which is the seam the shell did not have.
//

// ranCall is one recorded run of this binary.
type ranCall struct {
	args     []string
	combined bool

	// dir is the directory it ran in: empty for the server's own
	dir string
}

// recording answers every run with output and records what was asked.
func recording(output string) (*[]ranCall, func(string, []string, bool) []byte) {
	return scripted(func([]string) string { return output })
}

// scripted answers each run with what answer says for its arguments, and records what was asked.
func scripted(answer func(args []string) string) (*[]ranCall, func(string, []string, bool) []byte) {
	calls := &[]ranCall{}

	return calls, func(dir string, args []string, combined bool) []byte {
		*calls = append(*calls, ranCall{args: append([]string(nil), args...), combined: combined, dir: dir})

		return []byte(answer(args))
	}
}

func refusing(t *testing.T) func(string, []string, bool) []byte {
	return func(_ string, args []string, _ bool) []byte {
		t.Fatalf("ran %v, want nothing run", args)

		return nil
	}
}

func quiet(t *testing.T) {
	t.Helper()
	t.Setenv("HM_VERSION", "")
	t.Setenv("COMMAND_BIN_NAME", "")
	t.Setenv("HM_MCP_ROW_CAP", "")
	t.Setenv("HM_MCP_LOG_CAP", "")
	os.Unsetenv("HM_MCP_ROW_CAP")
	os.Unsetenv("HM_MCP_LOG_CAP")
}

func serving(server mcpServer, input string) string {
	out := &strings.Builder{}
	server.serve(strings.NewReader(input), out)

	return out.String()
}

func TestTheServerAnswersWhatTheShellScriptAnswered(t *testing.T) {
	for _, session := range []struct {
		name  string
		write bool
	}{{"read", false}, {"write", true}} {
		t.Run(session.name, func(t *testing.T) {
			quiet(t)

			requests, err := os.ReadFile(filepath.Join("testdata", "mcp", session.name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}

			golden, err := os.ReadFile(filepath.Join("testdata", "mcp", session.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}

			got := serving(mcpServer{write: session.write, capture: refusing(t)}, string(requests))

			if diff := cmp.Diff(strings.Split(string(golden), "\n"), strings.Split(got, "\n")); diff != "" {
				t.Fatalf("responses differ from the shell's (-shell +go):\n%s", diff)
			}
		})
	}
}

// answer is the text and error flag of the one tools/call result in out.
func answer(t *testing.T, out string) (string, bool) {
	t.Helper()

	var response struct {
		ID     int `json:"id"`
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}

	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v", out, err)
	}

	if len(response.Result.Content) != 1 || response.Result.Content[0].Type != "text" {
		t.Fatalf("content = %+v, want one text", response.Result.Content)
	}

	return response.Result.Content[0].Text, response.Result.IsError
}

func toolCall(name, arguments string) string {
	return `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"` + name +
		`","arguments":` + arguments + "}}\n"
}

func TestTheReadToolsAskThisBinaryForJSON(t *testing.T) {
	for tool, command := range map[string]string{
		"describe_project":   "describe",
		"list_environments":  "list",
		"check_environment":  "doctor",
		"environment_status": "status",
	} {
		t.Run(tool, func(t *testing.T) {
			quiet(t)
			calls, capture := recording(`{"ok":true,"schema_version":1,"data":{"project":"tienda","ports":[80,443],"empty":{}}}` + "\n\n")

			text, isError := answer(t, serving(mcpServer{capture: capture}, toolCall(tool, "{}")))

			if diff := cmp.Diff([]ranCall{{args: []string{command, "--json"}}}, *calls,
				cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}

			want := "{\n  \"project\": \"tienda\",\n  \"ports\": [\n    80,\n    443\n  ],\n  \"empty\": {}\n}"
			if text != want || isError {
				t.Fatalf("= %q, %v; want the data unwrapped, %q", text, isError, want)
			}
		})
	}
}

func TestWhatAReadToolMakesOfTheCommandsOutput(t *testing.T) {
	executable := "/opt/hm/bin/hm"
	unreadable := executable + " describe produced nothing readable. The environment may not be running: try check_environment."

	for _, c := range []struct {
		name, output, text string
		isError            bool
	}{
		{"nothing at all", "", unreadable, true},
		{"not JSON", "Error: no project\n", unreadable, true},
		{"null, which jq -e calls false", "null", unreadable, true},
		{"data that is a string is given raw", `{"ok":true,"data":"hola <&>\n\n"}`, "hola <&>", false},
		{"no data is the whole document", `{"ok":true,"n":1e2}`, "{\n  \"ok\": true,\n  \"n\": 1E+2\n}", false},
		// `.ok // true` is true for ok:false too, so the shell never took its error branch
		{"ok:false is passed on whole, as the shell did", `{"ok":false,"error":{"message":"m"}}`,
			"{\n  \"ok\": false,\n  \"error\": {\n    \"message\": \"m\"\n  }\n}", false},
		{"a document that is not an object is nothing", `[1]`, "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			quiet(t)
			_, capture := recording(c.output)

			text, isError := answer(t, serving(mcpServer{executable: executable, capture: capture},
				toolCall("describe_project", "{}")))

			if text != c.text || isError != c.isError {
				t.Fatalf("= %q, %v; want %q, %v", text, isError, c.text, c.isError)
			}
		})
	}
}

func TestServiceLogsReadsTheLogThroughThisBinary(t *testing.T) {
	for _, c := range []struct {
		name, arguments, cap string
		want                 []string
	}{
		{"a hundred lines unless asked", `{"service":"phpfpm"}`, "", []string{"100", "phpfpm"}},
		{"the lines asked for", `{"service":"db","lines":20}`, "", []string{"20", "db"}},
		{"a number in a string counts", `{"service":"db","lines":"30"}`, "", []string{"30", "db"}},
		{"anything else is the default", `{"service":"db","lines":-4}`, "", []string{"100", "db"}},
		{"and so is a number jq writes in scientific notation", `{"service":"db","lines":1e2}`, "", []string{"100", "db"}},
		{"capped at 500", `{"service":"db","lines":9000}`, "", []string{"500", "db"}},
		{"capped at HM_MCP_LOG_CAP", `{"service":"db","lines":90}`, "50", []string{"50", "db"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			quiet(t)
			if c.cap != "" {
				t.Setenv("HM_MCP_LOG_CAP", c.cap)
			}

			calls, capture := recording("línea 1\nlínea 2\n\n")

			text, isError := answer(t, serving(mcpServer{capture: capture}, toolCall("service_logs", c.arguments)))

			want := []ranCall{{args: append([]string{"--no-json", "logs", "--tail"}, c.want...), combined: true}}
			if diff := cmp.Diff(want, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}

			if text != "línea 1\nlínea 2" || isError {
				t.Fatalf("= %q, %v; want the log without its trailing newlines", text, isError)
			}
		})
	}

	t.Run("an empty log says so", func(t *testing.T) {
		quiet(t)
		_, capture := recording("\n")

		if text, _ := answer(t, serving(mcpServer{capture: capture},
			toolCall("service_logs", `{"service":"db"}`))); text != "(the log is empty)" {
			t.Fatalf("= %q", text)
		}
	})
}

func TestDatabaseQueryRunsWhatOnlyReads(t *testing.T) {
	t.Run("a select runs through mysql -q", func(t *testing.T) {
		quiet(t)
		calls, capture := recording("respuesta\n42\n")

		text, isError := answer(t, serving(mcpServer{capture: capture},
			toolCall("database_query", `{"sql":"SELECT 42 AS respuesta"}`)))

		want := []ranCall{{args: []string{"--no-json", "mysql", "-q", mcpReadOnly + "SELECT 42 AS respuesta"}, combined: true}}
		if diff := cmp.Diff(want, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
			t.Fatalf("runs (-want +got):\n%s", diff)
		}

		if text != "respuesta\n42" || isError {
			t.Fatalf("= %q, %v", text, isError)
		}
	})

	t.Run("a comment after the statement is ordinary", func(t *testing.T) {
		quiet(t)
		calls, capture := recording("1")

		serving(mcpServer{capture: capture}, toolCall("database_query", `{"sql":"show tables -- todas\n"}`))

		if len(*calls) != 1 || (*calls)[0].args[3] != mcpReadOnly+"show tables -- todas" {
			t.Fatalf("runs = %v, want the statement as given, its trailing newline gone", *calls)
		}
	})

	t.Run("no rows", func(t *testing.T) {
		quiet(t)
		_, capture := recording("")

		if text, _ := answer(t, serving(mcpServer{capture: capture},
			toolCall("database_query", `{"sql":"SELECT 1"}`))); text != "(no rows)" {
			t.Fatalf("= %q", text)
		}
	})

	t.Run("more rows than HM_MCP_ROW_CAP are cut", func(t *testing.T) {
		quiet(t)
		t.Setenv("HM_MCP_ROW_CAP", "2")
		_, capture := recording("a\nb\nc\n")

		text, _ := answer(t, serving(mcpServer{capture: capture}, toolCall("database_query", `{"sql":"SELECT 1"}`)))

		if want := "a\nb\n... truncated at 2 rows. Narrow the query with a WHERE or a LIMIT."; text != want {
			t.Fatalf("= %q, want %q", text, want)
		}
	})

	t.Run("two hundred unless told otherwise", func(t *testing.T) {
		quiet(t)
		_, capture := recording(strings.Repeat("fila\n", 201))

		text, _ := answer(t, serving(mcpServer{capture: capture}, toolCall("database_query", `{"sql":"SELECT 1"}`)))

		if !strings.HasSuffix(text, "fila\n... truncated at 200 rows. Narrow the query with a WHERE or a LIMIT.") ||
			strings.Count(text, "fila") != 200 {
			t.Fatalf("= %q", text)
		}
	})
}

// TestDatabaseQueryRunsInsideAReadOnlyTransaction: the text guard reads the statement, not what
// it does. A SELECT that calls a stored function which inserts a row passes it, and so does a
// locking read; the database itself has to refuse those, and it can only refuse them when the
// session and the transaction around the statement are read-only.
func TestDatabaseQueryRunsInsideAReadOnlyTransaction(t *testing.T) {
	for _, c := range []struct{ name, sql, statement string }{
		{"a plain select", "SELECT 1", "SELECT 1"},
		{"a function that writes, which the guard cannot see", "SELECT writes_a_row()", "SELECT writes_a_row()"},
		{"a locking read", "SELECT * FROM sales_order FOR UPDATE", "SELECT * FROM sales_order FOR UPDATE"},
		{"a trailing semicolon stays the last thing sent", "SELECT 1;", "SELECT 1;"},
		{"a trailing comment cannot swallow the wrapper", "show tables -- todas\n", "show tables -- todas"},
	} {
		t.Run(c.name, func(t *testing.T) {
			quiet(t)

			if refusal := mcpQueryRefusal(c.sql); refusal != "" {
				t.Fatalf("the guard refused %q (%s); this test is about what it lets through", c.sql, refusal)
			}

			calls, capture := recording("1")

			serving(mcpServer{capture: capture}, toolCall("database_query", `{"sql":`+jqText(c.sql).compact()+`}`))

			want := []ranCall{{args: []string{"--no-json", "mysql", "-q",
				"SET SESSION TRANSACTION READ ONLY;\nSTART TRANSACTION READ ONLY;\n" + c.statement}, combined: true}}
			if diff := cmp.Diff(want, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("the text guard still answers first, and nothing runs", func(t *testing.T) {
		quiet(t)

		text, isError := answer(t, serving(mcpServer{capture: refusing(t)},
			toolCall("database_query", `{"sql":"DELETE FROM core_config_data"}`)))

		if !isError || text != "Only SELECT, SHOW, DESCRIBE and EXPLAIN can be run here" {
			t.Fatalf("= %q, %v; want the guard's refusal", text, isError)
		}
	})
}

func TestTheWriteToolsRunOneMagentoCommandEach(t *testing.T) {
	for _, c := range []struct {
		tool, arguments string
		want            []string
	}{
		{"cache_flush", `{}`, []string{"cache:flush"}},
		{"cache_clean", `{}`, []string{"cache:clean"}},
		{"cache_clean", `{"types":["config","layout full_page"]}`, []string{"cache:clean", "config", "layout", "full_page"}},
		{"cache_clean", `{"types":"config"}`, []string{"cache:clean"}},
		{"reindex", `{}`, []string{"indexer:reindex"}},
		{"reindex", `{"indexer":"catalog_product_price"}`, []string{"indexer:reindex", "catalog_product_price"}},
		{"config_set", `{"path":"web/secure/use_in_frontend","value":"1"}`, []string{"config:set", "web/secure/use_in_frontend", "1"}},
		{"config_set", `{"path":"dev/js/merge_files"}`, []string{"config:set", "dev/js/merge_files", ""}},
	} {
		t.Run(c.tool+" "+c.arguments, func(t *testing.T) {
			quiet(t)
			calls, capture := recording("hecho\n")

			text, isError := answer(t, serving(mcpServer{write: true, capture: capture}, toolCall(c.tool, c.arguments)))

			want := []ranCall{{args: append([]string{"--no-json", "magento"}, c.want...), combined: true}}
			if diff := cmp.Diff(want, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}

			if text != "hecho" || isError {
				t.Fatalf("= %q, %v", text, isError)
			}
		})
	}

	t.Run("silence is done", func(t *testing.T) {
		quiet(t)
		_, capture := recording("")

		if text, _ := answer(t, serving(mcpServer{write: true, capture: capture},
			toolCall("cache_flush", "{}"))); text != "(done)" {
			t.Fatalf("= %q", text)
		}
	})

	t.Run("without --write they are not there and nothing runs", func(t *testing.T) {
		quiet(t)

		text, isError := answer(t, serving(mcpServer{capture: refusing(t)}, toolCall("cache_flush", "{}")))

		if !isError || !strings.HasPrefix(text, "There is no tool called 'cache_flush'.") {
			t.Fatalf("= %q, %v", text, isError)
		}
	})
}

func TestMCPConfigPrintsTheClientConfiguration(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	// The running binary itself, wherever it is installed: a release has no bin/ of its own
	binary := executable

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"mcp", "--config"}, `"mcp"`},
		{[]string{"mcp", "--config", "--write"}, `"mcp",` + "\n        " + `"--write"`},
		{[]string{"mcp", "--write", "--config"}, `"mcp",` + "\n        " + `"--write"`},
	} {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			answering(t)
			directory := here()

			stdout := &strings.Builder{}
			code := Run(c.args, stdout, io.Discard)

			want := "{\n  \"mcpServers\": {\n    \"hm\": {\n      \"command\": \"" + binary + "\",\n" +
				"      \"args\": [\n        " + c.want + "\n      ],\n" +
				"      \"cwd\": \"" + directory + "\"\n    }\n  }\n}\n"

			if code != exitOK || stdout.String() != want {
				t.Fatalf("= %d, %q; want 0, %q", code, stdout.String(), want)
			}
		})
	}
}

func TestMCPRefusesAnOptionItDoesNotKnow(t *testing.T) {
	answering(t)

	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	code := Run([]string{"mcp", "--config", "--nope", "--write"}, stdout, stderr)

	if code != exitUsage || stdout.String() != "" {
		t.Fatalf("= %d, stdout %q; want exitUsage and nothing printed", code, stdout.String())
	}

	var failed struct {
		OK    bool `json:"ok"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &failed); err != nil {
		t.Fatalf("stderr = %q, want the JSON failure: %v", stderr.String(), err)
	}

	if failed.OK || failed.Error.Type != "invalid_argument" || failed.Error.Message != "Unknown option: --nope" ||
		failed.Error.Hint != "hm mcp [--config] [--write]" {
		t.Fatalf("failure = %+v", failed)
	}
}

func TestMCPServesStandardInputUntilItEnds(t *testing.T) {
	answering(t)
	quiet(t)

	stdout := &strings.Builder{}
	code := mcp(nil, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"), stdout, io.Discard, true)

	if code != exitOK || stdout.String() != `{"jsonrpc":"2.0","id":1,"result":{}}`+"\n" {
		t.Fatalf("= %d, %q", code, stdout.String())
	}
}

// Inline rather than in the request file, which git would normalise: jq read the carriage return
// as whitespace, and the line was answered like any other.
func TestALineEndedByACarriageReturnIsAnswered(t *testing.T) {
	quiet(t)

	got := serving(mcpServer{capture: refusing(t)}, `{"jsonrpc":"2.0","id":12,"method":"ping"}`+"\r\n")

	if got != `{"jsonrpc":"2.0","id":12,"result":{}}`+"\n" {
		t.Fatalf("= %q", got)
	}
}

// TestABinaryThatCannotBeRunIsSaid: with nothing printed, every tool reads as success — "(done)"
// for a config:set that never ran. What stopped it is the answer instead.
func TestABinaryThatCannotBeRunIsSaid(t *testing.T) {
	output := string(mcpCapture(filepath.Join(t.TempDir(), "missing", "hm"), "", []string{"magento", "cache:flush"}, true))

	if !strings.Contains(output, "could not be run") {
		t.Errorf("output = %q, want it to say the binary could not be run", output)
	}
}

// TestACommandThatFailsSilentlyIsSaid: a command of this binary that exits with a failure and
// prints nothing would read as success — "(done)" for a config:set that failed. Its exit status is
// the answer instead.
func TestACommandThatFailsSilentlyIsSaid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hm")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, combined := range []bool{true, false} {
		output := string(mcpCapture(path, "", []string{"magento", "cache:flush"}, combined))

		if !strings.Contains(output, "exited with status 3") {
			t.Errorf("combined %v: output = %q, want the failing status said", combined, output)
		}
	}

	// A failure that says something is left to say it
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho broken\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if output := string(mcpCapture(path, "", nil, true)); output != "broken\n" {
		t.Errorf("output = %q, want what the command printed, untouched", output)
	}
}

// TestACommandForAnEnvironmentRunsInItsDirectory: a branch environment is a directory, and every
// command of this binary answers about the directory it runs in.
func TestACommandForAnEnvironmentRunsInItsDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hm")
	worktree := filepath.Join(root, "feature")

	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("#!/bin/sh\npwd -P\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	want, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}

	if output := string(mcpCapture(path, worktree, nil, false)); output != want+"\n" {
		t.Errorf("output = %q, want it run in %s", output, want)
	}

	if output := string(mcpCapture(path, filepath.Join(root, "gone"), nil, false)); !strings.Contains(output, "could not be run") {
		t.Errorf("a directory that is not there: output = %q, want it said", output)
	}
}

// instructions is what initialize told the client about the server.
func instructions(t *testing.T, server mcpServer) string {
	t.Helper()

	var response struct {
		Result struct {
			Instructions string `json:"instructions"`
		} `json:"result"`
	}

	out := serving(server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`+"\n")
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v", out, err)
	}

	return response.Result.Instructions
}

// TestTheInstructionsSayWhatTheToolsCanDo: a server started with --write that tells the model
// "nothing here changes the environment" is telling it the opposite of what four of its tools do.
func TestTheInstructionsSayWhatTheToolsCanDo(t *testing.T) {
	t.Run("without --write they are read-only, as they always said", func(t *testing.T) {
		quiet(t)

		want := "Read-only tools for the Magento environment in the current directory. " +
			"Nothing here changes the environment; use the hm command line for that."
		if got := instructions(t, mcpServer{capture: refusing(t)}); got != want {
			t.Fatalf("instructions = %q, want %q", got, want)
		}
	})

	t.Run("with --write they name the tools that change it", func(t *testing.T) {
		quiet(t)

		got := instructions(t, mcpServer{write: true, capture: refusing(t)})

		if strings.Contains(got, "Nothing here changes the environment") {
			t.Errorf("instructions = %q, want no claim that nothing changes", got)
		}

		for _, want := range []string{"cache_flush", "cache_clean", "reindex", "config_set", "change the environment"} {
			if !strings.Contains(got, want) {
				t.Errorf("instructions = %q, want them to mention %q", got, want)
			}
		}
	})
}
