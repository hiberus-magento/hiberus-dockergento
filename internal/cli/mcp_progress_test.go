package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// notifications/progress, as the 2025-06-18 revision this server answers in defines them
// (basic/utilities/progress): a request asks for them with params._meta.progressToken, a string or
// an integer; each notification carries that token, a progress that increases with every one, and
// an optional message; and they stop when the response is sent.
//

// streaming answers a run line by line, as the indexer prints one line per index rebuilt, and
// records what was asked.
func streaming(lines ...string) (*[]ranCall, func(string, []string, func(string)) []byte) {
	calls := &[]ranCall{}

	return calls, func(dir string, args []string, line func(string)) []byte {
		*calls = append(*calls, ranCall{args: append([]string(nil), args...), combined: true, dir: dir})

		for _, one := range lines {
			line(one)
		}

		return []byte(strings.Join(lines, "\n") + "\n")
	}
}

type progressNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		ProgressToken json.RawMessage `json:"progressToken"`
		Progress      float64         `json:"progress"`
		Message       string          `json:"message"`
	} `json:"params"`
}

func TestALongToolSendsProgressWhenTheRequestCarriesAToken(t *testing.T) {
	for _, token := range []string{`"reindex-1"`, `42`} {
		t.Run(token, func(t *testing.T) {
			quiet(t)

			calls, stream := streaming(
				"Design Config Grid index has been rebuilt successfully in 00:00:01",
				"Catalog Search index has been rebuilt successfully in 00:00:07",
			)

			request := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"reindex",` +
				`"arguments":{},"_meta":{"progressToken":` + token + `}}}` + "\n"

			out := serving(mcpServer{write: true, capture: refusing(t), stream: stream}, request)

			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(lines) != 4 {
				t.Fatalf("got %d lines, want three notifications and the response:\n%s", len(lines), out)
			}

			type seen struct {
				Token    string
				Progress float64
				Message  string
			}

			var got []seen

			for _, line := range lines[:3] {
				var notification progressNotification
				if err := json.Unmarshal([]byte(line), &notification); err != nil {
					t.Fatalf("json.Unmarshal(%q) = %v", line, err)
				}

				if notification.JSONRPC != "2.0" || notification.Method != "notifications/progress" ||
					strings.Contains(line, `"id"`) {
					t.Fatalf("%s is not a progress notification", line)
				}

				got = append(got, seen{string(notification.Params.ProgressToken),
					notification.Params.Progress, notification.Params.Message})
			}

			want := []seen{
				{token, 0, "Running hm magento indexer:reindex"},
				{token, 1, "Design Config Grid index has been rebuilt successfully in 00:00:01"},
				{token, 2, "Catalog Search index has been rebuilt successfully in 00:00:07"},
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("notifications (-want +got):\n%s", diff)
			}

			text, isError := answer(t, lines[3])
			if isError || !strings.Contains(text, "Catalog Search index has been rebuilt") {
				t.Fatalf("answer = %q (isError %v), want the indexer's output", text, isError)
			}

			if diff := cmp.Diff([]ranCall{{args: []string{"--no-json", "magento", "indexer:reindex"},
				combined: true}}, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("ran (-want +got):\n%s", diff)
			}
		})
	}
}

// Without a token nothing is sent but the response, and the command is captured as it always was.
func TestWithoutAProgressTokenOnlyTheResponseIsSent(t *testing.T) {
	quiet(t)

	calls, capture := recording("Catalog Search index has been rebuilt successfully in 00:00:07\n")
	_, stream := streaming("never")

	out := serving(mcpServer{write: true, capture: capture, stream: stream}, toolCall("reindex", "{}"))

	if strings.Count(out, "\n") != 1 || strings.Contains(out, "notifications/progress") {
		t.Fatalf("output = %q, want the response alone", out)
	}

	if len(*calls) != 1 {
		t.Fatalf("captured %d runs, want 1", len(*calls))
	}
}

// A tool that only reads answers at once: a token on it gets no notifications.
func TestAReadToolSendsNoProgress(t *testing.T) {
	quiet(t)

	_, capture := recording(`{"ok":true,"schema_version":1,"data":{}}` + "\n")
	_, stream := streaming("never")

	request := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"describe_project",` +
		`"arguments":{},"_meta":{"progressToken":"t"}}}` + "\n"

	if out := serving(mcpServer{capture: capture, stream: stream}, request); strings.Contains(out, "notifications/progress") {
		t.Fatalf("output = %q, want no notifications", out)
	}
}

// The real stream: every line reaches line as it is printed, both streams together, and a command
// that fails silently says so rather than reading as having worked.
func TestTheStreamHandsOverEveryLineAndKeepsTheOutput(t *testing.T) {
	dir := t.TempDir()

	script := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil { //nolint:gosec
			t.Fatal(err)
		}

		return path
	}

	var seen []string

	output := mcpStream(script("hm", "echo one\necho two >&2\nexit 3\n"), dir, []string{"magento"},
		func(line string) { seen = append(seen, line) })

	if diff := cmp.Diff([]string{"one", "two"}, seen); diff != "" {
		t.Fatalf("lines (-want +got):\n%s", diff)
	}

	if string(output) != "one\ntwo\n" {
		t.Fatalf("output = %q, want both lines", output)
	}

	silent := string(mcpStream(script("silent", "exit 3\n"), dir, []string{"magento"}, func(string) {}))
	if !strings.Contains(silent, "exited with status 3") {
		t.Fatalf("output = %q, want the status said", silent)
	}
}

// TestALineLongerThanAMebibyteIsKeptWhole: an indexer that prints a serialised object on one line
// is still answered with all of its output, that line included, and the lines after it are still
// handed over.
func TestALineLongerThanAMebibyteIsKeptWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hm")

	// 1.5 MiB of x on one line, then two ordinary ones
	body := "#!/bin/sh\nhead -c 1572864 /dev/zero | tr '\\0' x\necho\necho after\necho last >&2\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	var seen []string

	output := string(mcpStream(path, dir, []string{"magento"}, func(line string) { seen = append(seen, line) }))

	if want := strings.Repeat("x", 1572864) + "\nafter\nlast\n"; output != want {
		t.Fatalf("output = %d bytes ending %q, want the long line whole and the two after it (%d bytes)",
			len(output), output[max(0, len(output)-20):], len(want))
	}

	if len(seen) != 3 || len(seen[0]) != 1572864 || seen[1] != "after" || seen[2] != "last" {
		t.Fatalf("lines handed over = %d, want the long one and the two after it", len(seen))
	}
}
