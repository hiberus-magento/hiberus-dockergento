package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// What the server learned after the port: structured results, the stateless revision of the
// protocol (2026-07-28) next to the handshake ones, and one server for several branch
// environments.
//

// reply is one response line, decoded.
type reply struct {
	ID     any            `json:"id"`
	Result map[string]any `json:"result"`
	Error  *struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	} `json:"error"`
}

func decoded(t *testing.T, out string) reply {
	t.Helper()

	var response reply
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v", out, err)
	}

	return response
}

// modern is a request in the 2026-07-28 revision, which carries its version in every request.
func modern(id int, method, params string) string {
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientInfo":{"name":"prueba","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`

	if params == "" {
		params = "{" + meta + "}"
	} else {
		params = "{" + meta + "," + strings.TrimPrefix(params, "{")
	}

	return `{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"method":"` + method + `","params":` + params + "}\n"
}

func jsonNumber(n int) string {
	data, _ := json.Marshal(n)

	return string(data)
}

// tool is the tools/list entry called name.
func tool(t *testing.T, write bool, name string) map[string]any {
	t.Helper()

	response := decoded(t, serving(mcpServer{write: write, capture: refusing(t)},
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n"))

	for _, entry := range response.Result["tools"].([]any) {
		if entry := entry.(map[string]any); entry["name"] == name {
			return entry
		}
	}

	t.Fatalf("no tool called %s", name)

	return nil
}

func TestTheDataToolsAnswerWithStructuredContent(t *testing.T) {
	data := `{"project":{"name":"tienda"},"checks":[{"id":"a"}],"environments":[],"count":0}`

	for _, name := range []string{"describe_project", "list_environments", "check_environment", "environment_status"} {
		t.Run(name, func(t *testing.T) {
			quiet(t)

			schema, ok := tool(t, false, name)["outputSchema"].(map[string]any)
			if !ok || schema["type"] != "object" {
				t.Fatalf("outputSchema = %v, want an object schema (2025-06-18 clients require one)", schema)
			}

			_, capture := recording(`{"ok":true,"schema_version":1,"data":` + data + `}`)
			out := serving(mcpServer{capture: capture}, toolCall(name, "{}"))

			var want any
			if err := json.Unmarshal([]byte(data), &want); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(want, decoded(t, out).Result["structuredContent"]); diff != "" {
				t.Fatalf("structuredContent (-want +got):\n%s", diff)
			}

			// And the same data as text, for the clients that read only that
			if text, isError := answer(t, out); isError || !strings.Contains(text, `"tienda"`) {
				t.Fatalf("text = %q, %v; want the data serialised", text, isError)
			}
		})
	}

	t.Run("an answer that is not data has no structured content", func(t *testing.T) {
		quiet(t)
		_, capture := recording("Error: no project\n")

		result := decoded(t, serving(mcpServer{capture: capture}, toolCall("describe_project", "{}"))).Result

		if _, there := result["structuredContent"]; there || result["isError"] != true {
			t.Fatalf("result = %v, want an error with no structuredContent", result)
		}
	})

	t.Run("the other tools answer in text alone, as before", func(t *testing.T) {
		quiet(t)

		for _, name := range []string{"service_logs", "database_query"} {
			if _, there := tool(t, false, name)["outputSchema"]; there {
				t.Errorf("%s has an outputSchema, want none: its answer is not JSON", name)
			}
		}
	})
}

func TestServerDiscoverSaysWhatTheServerSpeaks(t *testing.T) {
	for _, c := range []struct {
		name, request string
	}{
		{"asked in 2026-07-28", modern(1, "server/discover", "")},
		// A dual-era client probes before it knows anything, and may not send a version
		{"asked with no version", `{"jsonrpc":"2.0","id":1,"method":"server/discover"}` + "\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			quiet(t)
			t.Setenv("HM_VERSION", "9.9.9")

			result := decoded(t, serving(mcpServer{capture: refusing(t)}, c.request)).Result

			want := map[string]any{
				"resultType":        "complete",
				"supportedVersions": []any{"2026-07-28", "2025-06-18", "2025-03-26", "2024-11-05"},
				"capabilities":      map[string]any{"tools": map[string]any{}},
				"_meta": map[string]any{
					"io.modelcontextprotocol/serverInfo": map[string]any{"name": "hm", "version": "9.9.9"},
				},
				"instructions": mcpInstructions,
				"ttlMs":        float64(300000),
				"cacheScope":   "private",
			}

			if diff := cmp.Diff(want, result); diff != "" {
				t.Fatalf("server/discover (-want +got):\n%s", diff)
			}
		})
	}
}

func TestARequestInTheStatelessRevisionIsServedInIt(t *testing.T) {
	t.Run("tools/list is a complete, cacheable result", func(t *testing.T) {
		quiet(t)

		result := decoded(t, serving(mcpServer{capture: refusing(t)}, modern(2, "tools/list", ""))).Result

		if result["resultType"] != "complete" || result["ttlMs"] != float64(300000) || result["cacheScope"] != "private" {
			t.Fatalf("result = %v, want resultType, ttlMs and cacheScope", result)
		}

		if tools, _ := result["tools"].([]any); len(tools) != len(mcpToolNames(false)) {
			t.Fatalf("tools = %v, want the read-only tools", result["tools"])
		}
	})

	t.Run("tools/call says it is complete and who answered", func(t *testing.T) {
		quiet(t)
		t.Setenv("HM_VERSION", "9.9.9")
		_, capture := recording(`{"ok":true,"data":{"count":0}}`)

		result := decoded(t, serving(mcpServer{capture: capture},
			modern(3, "tools/call", `{"name":"list_environments","arguments":{}}`))).Result

		meta, _ := result["_meta"].(map[string]any)
		if result["resultType"] != "complete" || meta["io.modelcontextprotocol/serverInfo"] == nil {
			t.Fatalf("result = %v, want resultType and serverInfo", result)
		}

		if diff := cmp.Diff(map[string]any{"count": float64(0)}, result["structuredContent"]); diff != "" {
			t.Fatalf("structuredContent (-want +got):\n%s", diff)
		}
	})

	t.Run("a version it does not speak is refused with the ones it does", func(t *testing.T) {
		quiet(t)

		request := `{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01"}}}` + "\n"
		response := decoded(t, serving(mcpServer{capture: refusing(t)}, request))

		if response.Error == nil || response.Error.Code != -32022 || response.Error.Message != "Unsupported protocol version" {
			t.Fatalf("response = %+v, want UnsupportedProtocolVersionError", response)
		}

		want := map[string]any{
			"supported": []any{"2026-07-28", "2025-06-18", "2025-03-26", "2024-11-05"},
			"requested": "1900-01-01",
		}
		if diff := cmp.Diff(want, response.Error.Data); diff != "" {
			t.Fatalf("data (-want +got):\n%s", diff)
		}
	})

	t.Run("a request with no version is answered as it always was", func(t *testing.T) {
		quiet(t)

		got := serving(mcpServer{capture: refusing(t)}, `{"jsonrpc":"2.0","id":9,"method":"ping"}`+"\n")

		if got != `{"jsonrpc":"2.0","id":9,"result":{}}`+"\n" {
			t.Fatalf("= %q, want no resultType: a 2025 client never asked for one", got)
		}
	})

	t.Run("initialize still answers in the handshake versions", func(t *testing.T) {
		quiet(t)

		result := decoded(t, serving(mcpServer{capture: refusing(t)},
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`+"\n")).Result

		if result["protocolVersion"] != "2025-03-26" {
			t.Fatalf("protocolVersion = %v, want the client's own", result["protocolVersion"])
		}
	})
}

// worktrees is what `hm worktree list --json` answers in a project with two branch environments.
const worktrees = `{"ok":true,"schema_version":1,"data":{"project":"tienda","worktrees":[` +
	`{"name":"feature-x","branch":"feature/x","profile":"full","state":"running","url":"https://feature-x.tienda.local","path":"/src/tienda-feature-x"},` +
	`{"name":"borrado","branch":"old","profile":"full","state":"missing","url":"https://borrado.tienda.local","path":"/src/borrado"}]}}`

func inEnvironments(answer string) func([]string) string {
	return func(args []string) string {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "list" {
			return worktrees
		}

		return answer
	}
}

func TestTheEnvironmentArgumentPicksABranchEnvironment(t *testing.T) {
	lookup := ranCall{args: []string{"worktree", "list", "--json"}}

	for _, c := range []struct {
		tool, arguments string
		write           bool
		want            ranCall
	}{
		{"describe_project", `{"environment":"feature-x"}`, false, ranCall{args: []string{"describe", "--json"}}},
		{"check_environment", `{"environment":"feature-x"}`, false, ranCall{args: []string{"doctor", "--json"}}},
		{"service_logs", `{"service":"db","environment":"feature-x"}`, false,
			ranCall{args: []string{"--no-json", "logs", "--tail", "100", "db"}, combined: true}},
		{"database_query", `{"sql":"SELECT 1","environment":"feature-x"}`, false,
			ranCall{args: []string{"--no-json", "mysql", "-q", mcpReadOnly + "SELECT 1"}, combined: true}},
		{"cache_flush", `{"environment":"feature-x"}`, true,
			ranCall{args: []string{"--no-json", "magento", "cache:flush"}, combined: true}},
	} {
		t.Run(c.tool, func(t *testing.T) {
			quiet(t)
			calls, capture := scripted(inEnvironments(`{"ok":true,"data":{"n":1}}`))

			_, isError := answer(t, serving(mcpServer{write: c.write, capture: capture, captureStatus: withStatus(capture)}, toolCall(c.tool, c.arguments)))

			c.want.dir = "/src/tienda-feature-x"
			if diff := cmp.Diff([]ranCall{lookup, c.want}, *calls, cmp.AllowUnexported(ranCall{})); diff != "" || isError {
				t.Fatalf("isError %v; runs (-want +got):\n%s", isError, diff)
			}

			if _, there := tool(t, c.write, c.tool)["inputSchema"].(map[string]any)["properties"].(map[string]any)["environment"]; !there {
				t.Errorf("%s does not declare an environment argument", c.tool)
			}
		})
	}

	t.Run("without it the tools run where the server runs, as before", func(t *testing.T) {
		quiet(t)
		calls, capture := recording(`{"ok":true,"data":{"n":1}}`)

		serving(mcpServer{capture: capture}, toolCall("describe_project", "{}"))

		if diff := cmp.Diff([]ranCall{{args: []string{"describe", "--json"}}}, *calls,
			cmp.AllowUnexported(ranCall{})); diff != "" {
			t.Fatalf("runs (-want +got):\n%s", diff)
		}
	})

	t.Run("list_environments is about the machine and takes none", func(t *testing.T) {
		quiet(t)

		if _, there := tool(t, false, "list_environments")["inputSchema"].(map[string]any)["properties"].(map[string]any)["environment"]; there {
			t.Error("list_environments declares an environment argument, want none")
		}
	})

	for _, c := range []struct{ name, environment, want string }{
		{"an unknown name is a tool error that names the ones there are", "nada",
			"There is no branch environment called 'nada' in this project. The ones there are: feature-x borrado"},
		{"one whose directory is gone says so", "borrado",
			"The branch environment 'borrado' no longer has its directory (/src/borrado). Collect it with hm clean."},
	} {
		t.Run(c.name, func(t *testing.T) {
			quiet(t)
			calls, capture := scripted(inEnvironments("should not run"))

			text, isError := answer(t, serving(mcpServer{capture: capture},
				toolCall("describe_project", `{"environment":"`+c.environment+`"}`)))

			if text != c.want || !isError {
				t.Fatalf("= %q, %v; want %q, an error", text, isError, c.want)
			}

			if diff := cmp.Diff([]ranCall{lookup}, *calls, cmp.AllowUnexported(ranCall{})); diff != "" {
				t.Fatalf("runs (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("a project whose environments cannot be read says so", func(t *testing.T) {
		quiet(t)
		_, capture := recording("Error: not a project\n")

		text, isError := answer(t, serving(mcpServer{capture: capture},
			toolCall("describe_project", `{"environment":"feature-x"}`)))

		if !isError || !strings.HasPrefix(text, "The branch environments of this project could not be read") {
			t.Fatalf("= %q, %v", text, isError)
		}
	})

	t.Run("an unknown tool is still an unknown tool", func(t *testing.T) {
		quiet(t)

		text, isError := answer(t, serving(mcpServer{capture: refusing(t)},
			toolCall("nada", `{"environment":"feature-x"}`)))

		if !isError || !strings.HasPrefix(text, "There is no tool called 'nada'.") {
			t.Fatalf("= %q, %v", text, isError)
		}
	})
}
