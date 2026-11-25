package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

//
// Progress for the tools that take a while.
//
// notifications/progress as the 2025-06-18 revision defines it
// (https://modelcontextprotocol.io/specification/2025-06-18/basic/utilities/progress), unchanged in
// 2026-07-28 (the same path under specification/2026-07-28): a request
// asks for it with params._meta.progressToken, a string or an integer; each notification carries
// that token, a progress that increases with every one and a message; and they stop once the
// response is sent, which here they do by construction, since a call is answered only after its
// command has ended.
//
// What is reported is what the command prints, line by line, as it prints it: the indexer says one
// line per index rebuilt, which is the only measure of how far it has got that there is. No total
// is sent, because nothing knows it.
//

// mcpLongTools are the tools whose answer can take minutes. The others answer in a moment, and a
// notification for them would only be noise.
var mcpLongTools = map[string]bool{"reindex": true}

// reporting is the server for one call, with progress on when that call is long and asked for it.
func (s mcpServer) reporting(name string, params jqValue) mcpServer {
	if !mcpLongTools[name] || s.stream == nil || s.out == nil {
		return s
	}

	meta, ok := params.field("_meta")
	if !ok || meta.kind != jqObject {
		return s
	}

	token, _ := meta.field("progressToken")

	// A string or an integer, and nothing else is a token
	if token.kind != jqString && (token.kind != jqNumber || strings.ContainsAny(token.text, ".eE")) {
		return s
	}

	out, sent := s.out, 0

	s.progress = func(message string) {
		if message = strings.TrimSpace(message); message == "" {
			return
		}

		notification := jqObjectOf("jsonrpc", jqText("2.0"), "method", jqText("notifications/progress"),
			"params", jqObjectOf(
				"progressToken", token,
				"progress", jqValue{kind: jqNumber, text: strconv.Itoa(sent)},
				"message", jqText(message),
			))

		io.WriteString(out, notification.compact()+"\n") //nolint:errcheck

		sent++
	}

	return s
}

// followed runs a command of this binary with its output reported as it arrives, and answers
// with all of it, as run would.
func (s mcpServer) followed(args ...string) string {
	shown := args
	if len(shown) > 0 && shown[0] == "--no-json" {
		shown = shown[1:]
	}

	s.progress("Running " + binaryName() + " " + strings.Join(shown, " "))

	return substituted(string(s.stream(s.dir, args, s.progress)))
}

// mcpStream is mcpCapture, combined, with every line handed to line as it is printed. dir is a
// branch environment's directory, or empty for the server's own.
var mcpStream = func(executable, dir string, args []string, line func(string)) []byte {
	reader, writer := io.Pipe()

	command := exec.Command(executable, args...)
	command.Dir = dir
	command.Stdout = writer
	command.Stderr = writer

	var output bytes.Buffer

	read := make(chan struct{})

	go func() {
		defer close(read)

		// A reader rather than a Scanner: a Scanner stops at a line longer than its buffer and
		// drops what it had read of it, and a serialised object on one line is longer than any
		// buffer worth picking. Reading to the end also means the command never blocks on a pipe
		// nobody empties.
		lines := bufio.NewReader(reader)

		for {
			text, err := lines.ReadString('\n')
			if text != "" {
				text = strings.TrimSuffix(text, "\n")
				output.WriteString(text + "\n")
				line(text)
			}

			if err != nil {
				break
			}
		}
	}()

	err := command.Run()
	writer.Close() //nolint:errcheck
	<-read

	status := 0

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		status, err = exit.ExitCode(), nil
	}

	// Said, for the reason mcpCapture says it: an empty answer reads as a change that worked
	switch {
	case err != nil:
		output.WriteString(executable + " could not be run: " + err.Error())
	case status != 0 && strings.TrimSpace(output.String()) == "":
		output.WriteString(fmt.Sprintf("%s %s exited with status %d and printed nothing",
			binaryName(), strings.Join(args, " "), status))
	}

	return output.Bytes()
}
