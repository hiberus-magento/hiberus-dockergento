package cli

import (
	"bufio"
	"fmt"
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
// exist only with --write, and are absent rather than refused without it; the two read tools that
// can also act (xdebug, after_change) say without it that acting takes --write.
//

const (
	mcpVersion      = "2025-06-18"
	mcpInstructions = "Read-only tools for the Magento environment in the current directory. Nothing here changes the environment; use the hm command line for that."
)

// mcpKnownVersions are the handshake revisions, the ones `initialize` negotiates.
var mcpKnownVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// mcpStatelessVersion is the revision with no handshake, where each request names its version.
const mcpStatelessVersion = "2026-07-28"

// instructions is what the client is told about this server. With --write, saying that nothing
// changes the environment would be the opposite of what the write tools do, so they are named.
func (s mcpServer) instructions() string {
	if !s.write {
		return mcpInstructions
	}

	// Read from the catalogue's own annotations, so a tool that becomes able to change something
	// is named here without anybody editing a sentence
	var writing []string

	for _, tool := range mcpCatalogue(true).items {
		hints, _ := tool.field("annotations")
		if readOnly, _ := hints.field("readOnlyHint"); readOnly.kind == jqFalse {
			name, _ := tool.field("name")
			writing = append(writing, name.text)
		}
	}

	return "Tools for the Magento environment in the current directory. " +
		strings.Join(writing, ", ") + " can change the environment; the others only read it. " +
		"after_change runs setup:upgrade only when asked to. " +
		"hm setup, Composer, database imports and anything that removes an environment are not offered: use the hm command line for those."
}

// mcpCapture runs this tool's binary with args, standard input closed, and returns what it
// printed: standard output alone, or together with standard error when combined. It is what a
// test replaces, so that no real command runs. A binary that cannot be started says so in the
// output: left empty, every tool would read as having worked — "(done)" for a change never made.
// A command that fails and prints nothing would read the same way, so its exit status is said
// (the shell's command substitution dropped it: a deliberate difference).
//
// dir is the directory it runs in, a branch environment's, or empty for the server's own.
var mcpCapture = func(executable, dir string, args []string, combined bool) []byte {
	output, status, err := selfrun.Runner{Executable: executable, Dir: dir}.CaptureStatus(args, combined)

	switch {
	case err != nil:
		output = append(output, []byte(executable+" could not be run: "+err.Error())...)
	case status != 0 && strings.TrimSpace(string(output)) == "":
		output = []byte(fmt.Sprintf("%s %s exited with status %d and printed nothing",
			binaryName(), strings.Join(args, " "), status))
	}

	return output
}

// mcpCaptureStatus is mcpCapture with the exit status, for the tools whose answer depends on it:
// a test run that failed, a step that has to stop the ones after it. A binary that cannot be
// started says so in the output, with -1 as its status.
var mcpCaptureStatus = func(executable, dir string, args []string, combined bool) ([]byte, int) {
	output, status, err := selfrun.Runner{Executable: executable, Dir: dir}.CaptureStatus(args, combined)
	if err != nil {
		return append(output, []byte(executable+" could not be run: "+err.Error())...), -1
	}

	return output, status
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

	executable := executablePath()

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
		capture: func(dir string, args []string, combined bool) []byte {
			return mcpCapture(executable, dir, args, combined)
		},
		captureStatus: func(dir string, args []string, combined bool) ([]byte, int) {
			return mcpCaptureStatus(executable, dir, args, combined)
		},
		stream: func(dir string, args []string, line func(string)) []byte {
			return mcpStream(executable, dir, args, line)
		},
	}
	server.serve(in, stdout)

	return exitOK
}

// toolRoot is the installation this binary belongs to, the directory bin/ is in: what the shell
// called COMMAND_BIN_DIR, worked out the same way, with every symlink resolved.
//
// A variable so that a test can point it at a tool of its own.
var toolRoot = func() string {
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

	// capture runs that binary in a directory: see mcpCapture
	capture func(dir string, args []string, combined bool) []byte

	// captureStatus is capture with the exit status: see mcpCaptureStatus
	captureStatus func(dir string, args []string, combined bool) ([]byte, int)

	// dir is where one tool call runs its commands: the branch environment its `environment`
	// argument named, or empty for the directory the server was started in
	dir string

	// stream, out and progress are for the long tools: see mcp_progress.go
	stream   func(dir string, args []string, line func(string)) []byte
	out      io.Writer
	progress func(message string)
}

// serve answers one line at a time until in ends.
//
// A last line with no newline after it is never answered: the shell read lines with `read`,
// which drops one, and an MCP client ends every message with a newline anyway.
func (s mcpServer) serve(in io.Reader, out io.Writer) {
	reader := bufio.NewReader(in)
	s.out = out

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

	//
	// The 2026-07-28 revision has no handshake: every request says which version it speaks, in
	// _meta, and is answered in it or refused. A request that says nothing is a client of the
	// handshake revisions, and is answered exactly as before — this server speaks both.
	//
	stateless := false

	if meta, ok := params.field("_meta"); ok && meta.kind == jqObject {
		if requested, _ := meta.field("io.modelcontextprotocol/protocolVersion"); requested.kind != jqNull {
			if !mcpSupports(requested) {
				return mcpUnsupported(id, requested), true
			}

			stateless = requested.text == mcpStatelessVersion
		}
	}

	result := func(value jqValue) (string, bool) {
		if stateless {
			value = value.with("resultType", jqText("complete")).
				with("_meta", jqObjectOf("io.modelcontextprotocol/serverInfo", mcpServerInfo()))
		}

		return mcpResult(id, value), true
	}

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

		return mcpResult(id, jqObjectOf(
			"protocolVersion", jqText(version),
			"capabilities", jqObjectOf("tools", jqObjectOf()),
			"serverInfo", mcpServerInfo(),
			"instructions", jqText(s.instructions()),
		)), true
	case "server/discover":
		// Asked before anything else, by a client finding out which revision this server speaks,
		// so it is answered in the stateless one whether the request named a version or not
		stateless = true

		versions := []jqValue{jqText(mcpStatelessVersion)}
		for _, known := range mcpKnownVersions {
			versions = append(versions, jqText(known))
		}

		return result(mcpCacheable(jqObjectOf(
			"supportedVersions", jqValue{kind: jqArray, items: versions},
			"capabilities", jqObjectOf("tools", jqObjectOf()),
			"instructions", jqText(s.instructions()),
		)))
	case "tools/list":
		if stateless {
			return result(mcpCacheable(jqObjectOf("tools", mcpCatalogue(s.write))))
		}

		return result(jqObjectOf("tools", mcpCatalogue(s.write)))
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

		return result(s.reporting(name, params).call(name, arguments))
	case "ping":
		return result(jqObjectOf())
	case "":
		return mcpFail(id, -32600, "Invalid request: no method"), true
	}

	return mcpFail(id, -32601, "This server does not implement '"+method+"'"), true
}

// mcpSupports says whether a request's protocol version is one this server speaks.
func mcpSupports(requested jqValue) bool {
	if requested.kind != jqString {
		return false
	}

	if requested.text == mcpStatelessVersion {
		return true
	}

	for _, known := range mcpKnownVersions {
		if requested.text == known {
			return true
		}
	}

	return false
}

// mcpUnsupported is UnsupportedProtocolVersionError, which lists the versions there are so that
// the client can retry in one of them.
func mcpUnsupported(id, requested jqValue) string {
	supported := []jqValue{jqText(mcpStatelessVersion)}
	for _, known := range mcpKnownVersions {
		supported = append(supported, jqText(known))
	}

	return jqObjectOf("jsonrpc", jqText("2.0"), "id", id, "error", jqObjectOf(
		"code", jqValue{kind: jqNumber, text: "-32022"},
		"message", jqText("Unsupported protocol version"),
		"data", jqObjectOf("supported", jqValue{kind: jqArray, items: supported}, "requested", requested),
	)).compact()
}

// mcpCacheable is a result the client may keep. What this server offers is fixed for as long as it
// runs, by --write; the time is short because a server restarted with the other flag answers
// differently, and private because it describes one machine.
func mcpCacheable(result jqValue) jqValue {
	return result.with("ttlMs", jqValue{kind: jqNumber, text: "300000"}).with("cacheScope", jqText("private"))
}

// mcpServerInfo is who is answering.
func mcpServerInfo() jqValue {
	toolVersion := os.Getenv("HM_VERSION")
	if toolVersion == "" {
		toolVersion = Version
	}

	if toolVersion == "" {
		toolVersion = "unknown"
	}

	return jqObjectOf("name", jqText(binaryName()), "version", jqText(toolVersion))
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
