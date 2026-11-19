package cli

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/selfrun"
)

//
// The Model Context Protocol, spoken over stdin and stdout.
//
// Ported from console/commands/mcp.sh and the tasks it sources (console/tasks/mcp_tools.sh), and
// answering byte for byte what they answered: the same messages, the same catalogue, the same
// refusals, the same `--config` document — except that the document names this binary rather
// than bin/run, which is what `ai-context` already writes into .mcp.json.
//
// Everything printed on stdout here is a protocol message. Every command called has its standard
// input closed and, where the shell redirected it, its standard error dropped: one stray warning
// on stdout is a parse error in the client, and one command reading stdin would eat the
// conversation.
//
// The read-only half and the write half are separate decisions: `hm exec` and `hm mysql` run
// whatever they are given, and a list of typed questions is the opposite shape. The write tools
// exist only with --write, and are absent rather than refused without it.
//

const (
	mcpVersion      = "2025-06-18"
	mcpInstructions = "Read-only tools for the Magento environment in the current directory. Nothing here changes the environment; use the hm command line for that."
)

var mcpKnownVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// mcpCapture runs this tool's binary with args, standard input closed, and returns what it
// printed: standard output alone, or together with standard error when combined. It is what a
// test replaces, so that no real command runs.
var mcpCapture = func(executable string, args []string, combined bool) []byte {
	output, _ := selfrun.Runner{Executable: executable}.Capture(args, combined)

	return output
}

// mcp is the command: --config prints the client configuration, anything else serves the
// protocol on in until it ends.
func mcp(args []string, in io.Reader, stdout, stderr io.Writer, jsonOutput bool) int {
	showConfig, write := false, false

	for _, argument := range args {
		switch argument {
		case "--config":
			showConfig = true
		case "--write":
			write = true
		default:
			return failure(stderr, jsonOutput, "mcp", exitUsage, "invalid_argument",
				"Unknown option: "+argument, binaryName()+" mcp [--config] [--write]")
		}
	}

	executable := filepath.Join(toolRoot(), "bin", "hm")

	if showConfig {
		//
		// Printed, not written. An MCP client's configuration has other servers in it, and
		// editing somebody's file to save them a copy and paste is not a good trade.
		//
		arguments := jqValue{kind: jqArray, items: []jqValue{jqText("mcp")}}
		if write {
			arguments.items = append(arguments.items, jqText("--write"))
		}

		document := jqObjectOf("mcpServers", jqObjectOf("hm", jqObjectOf(
			"command", jqText(executable), "args", arguments, "cwd", jqText(here()))))

		io.WriteString(stdout, document.pretty()+"\n") //nolint:errcheck

		return exitOK
	}

	server := mcpServer{
		write:      write,
		executable: executable,
		capture: func(args []string, combined bool) []byte {
			return mcpCapture(executable, args, combined)
		},
	}
	server.serve(in, stdout)

	return exitOK
}

// toolRoot is the installation this binary belongs to, the directory bin/ is in: what the shell
// called COMMAND_BIN_DIR, worked out the same way, with every symlink resolved.
func toolRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	return filepath.Dir(filepath.Dir(executable))
}

// mcpServer is one session.
type mcpServer struct {
	write bool

	// executable is the binary tool calls run, named in the answer when one prints nothing
	executable string

	// capture runs that binary: see mcpCapture
	capture func(args []string, combined bool) []byte
}

// serve answers one line at a time until in ends.
//
// A last line with no newline after it is never answered: the shell read lines with `read`,
// which drops one, and an MCP client ends every message with a newline anyway.
func (s mcpServer) serve(in io.Reader, out io.Writer) {
	reader := bufio.NewReader(in)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			continue
		}

		if response, ok := s.handle(line); ok {
			io.WriteString(out, response+"\n") //nolint:errcheck
		}
	}
}

// handle answers one line, or says it is not to be answered.
func (s mcpServer) handle(line string) (string, bool) {
	request, ok := parseJQ([]byte(line))
	if !ok || !request.truthy() {
		return mcpFail(jqValue{kind: jqNull}, -32700, "Parse error: that line is not JSON"), true
	}

	//
	// A document that is not an object: the shell's jq stopped on `.id` and printed nothing, and
	// what went on the wire was the empty line it then sent
	//
	if request.kind != jqObject {
		return "", true
	}

	id := jqValue{kind: jqNull}
	if value, _ := request.field("id"); value.truthy() {
		id = value
	}

	method := rawField(request, "method", "")

	// A notification has no id and must never be answered
	if id.kind == jqNull && method != "" {
		return "", false
	}

	params, _ := request.field("params")

	switch method {
	case "initialize":
		requested := rawField(params, "protocolVersion", "")
		version := mcpVersion

		// Answer in the client's version when it is one we know, which is what keeps an older
		// client talking to a newer server
		for _, known := range mcpKnownVersions {
			if requested == known {
				version = requested
			}
		}

		name := binaryName()

		toolVersion := os.Getenv("HM_VERSION")
		if toolVersion == "" {
			toolVersion = "unknown"
		}

		return mcpResult(id, jqObjectOf(
			"protocolVersion", jqText(version),
			"capabilities", jqObjectOf("tools", jqObjectOf()),
			"serverInfo", jqObjectOf("name", jqText(name), "version", jqText(toolVersion)),
			"instructions", jqText(mcpInstructions),
		)), true
	case "tools/list":
		return mcpResult(id, jqObjectOf("tools", mcpCatalogue(s.write))), true
	case "tools/call":
		name := rawField(params, "name", "")

		arguments, ok := params.field("arguments")
		if ok && !arguments.truthy() {
			arguments = jqObjectOf()
		}

		if !ok {
			// jq stopped on it, and the shell handed the tool an empty string
			arguments = jqValue{kind: jqString}
		}

		return mcpResult(id, s.call(name, arguments)), true
	case "ping":
		return mcpResult(id, jqObjectOf()), true
	case "":
		return mcpFail(id, -32600, "Invalid request: no method"), true
	}

	return mcpFail(id, -32601, "This server does not implement '"+method+"'"), true
}

func mcpResult(id, result jqValue) string {
	return jqObjectOf("jsonrpc", jqText("2.0"), "id", id, "result", result).compact()
}

func mcpFail(id jqValue, code int, message string) string {
	return jqObjectOf("jsonrpc", jqText("2.0"), "id", id, "error", jqObjectOf(
		"code", jqValue{kind: jqNumber, text: strconv.Itoa(code)}, "message", jqText(message))).compact()
}

// mcpContent is a tool's answer. A tool that could not answer returns a result, not a protocol
// error: a model can read the sentence and try something else, where a JSON-RPC error is
// surfaced by most clients as a broken server and ends the conversation.
func mcpContent(text string, isError bool) jqValue {
	return jqObjectOf(
		"content", jqValue{kind: jqArray, items: []jqValue{jqObjectOf("type", jqText("text"), "text", jqText(text))}},
		"isError", jqBool(isError),
	)
}

// rawField is `$(jq -r '.key // fallback')`: the value raw, with the trailing newlines a command
// substitution strips, and nothing when the value is not an object and jq stopped on it.
func rawField(value jqValue, key, fallback string) string {
	field, ok := value.field(key)
	if !ok {
		return ""
	}

	if !field.truthy() {
		return fallback
	}

	return substituted(field.raw())
}

// substituted is what `$(...)` keeps of a command's output: all of it but the trailing newlines.
func substituted(output string) string {
	return strings.TrimRight(output, "\n")
}
