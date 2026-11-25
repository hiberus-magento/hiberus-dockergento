package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func shopStatus() core.Status {
	return core.Status{
		Project: "shop", Root: "/code/shop", State: "partial", Mode: "developer", Xdebug: "off",
		Anonymised: "yes", AnonymisedAt: "2026-11-25 10:00",
		Services: []core.ServiceStatus{
			{Name: "db", State: "running", Running: true},
			{Name: "nginx", State: "running", Running: true, LastError: "2026/11/25 [error] upstream timed out"},
			{Name: "search", State: "exited"},
		},
		URLs: []core.URLStatus{
			{Name: "base", URL: "https://shop.local/", Answers: true, Code: 200},
			{Name: "mail", URL: "http://localhost:8025", Error: "connection refused"},
		},
	}
}

func TestStatusAsksTheEngineAboutThisDirectory(t *testing.T) {
	fake := answering(t)
	fake.status = shopStatus()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if code := Run([]string{"status"}, &strings.Builder{}, &strings.Builder{}); code != exitOK {
		t.Fatalf("status = %d, want exitOK", code)
	}

	if diff := cmp.Diff([]call{{Method: "Status", Dir: cwd}}, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestStatusAsJSONIsOneEnvelope decodes into a hand-written shape, so a renamed tag is a failure
// rather than the encoder agreeing with itself.
func TestStatusAsJSONIsOneEnvelope(t *testing.T) {
	fake := answering(t)
	fake.status = shopStatus()

	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	if code := Run([]string{"status", "--json"}, stdout, stderr); code != exitOK {
		t.Fatalf("status --json = %d, stderr %q", code, stderr.String())
	}

	var document struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Project      string `json:"project"`
			State        string `json:"state"`
			Mode         string `json:"mode"`
			Xdebug       string `json:"xdebug"`
			Anonymised   string `json:"anonymised"`
			AnonymisedAt string `json:"anonymised_at"`
			Services     []struct {
				Name      string `json:"name"`
				State     string `json:"state"`
				Running   bool   `json:"running"`
				LastError string `json:"last_error"`
			} `json:"services"`
			URLs []map[string]any `json:"urls"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(stdout.String()), &document); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v", stdout.String(), err)
	}

	if !document.OK || document.Command != "status" || document.Data.Project != "shop" ||
		document.Data.State != "partial" || document.Data.Mode != "developer" || document.Data.Xdebug != "off" ||
		document.Data.Anonymised != "yes" || document.Data.AnonymisedAt != "2026-11-25 10:00" {
		t.Fatalf("document = %+v", document)
	}

	if len(document.Data.Services) != 3 || document.Data.Services[1].LastError != "2026/11/25 [error] upstream timed out" ||
		!document.Data.Services[1].Running || document.Data.Services[2].Running {
		t.Fatalf("services = %+v", document.Data.Services)
	}

	want := []map[string]any{
		{"name": "base", "url": "https://shop.local/", "answers": true, "code": float64(200)},
		{"name": "mail", "url": "http://localhost:8025", "answers": false, "error": "connection refused"},
	}
	if diff := cmp.Diff(want, document.Data.URLs); diff != "" {
		t.Fatalf("urls (-want +got):\n%s", diff)
	}

	if stderr.String() != "" {
		t.Errorf("stderr = %q, want nothing", stderr.String())
	}
}

func TestStatusAsTextIsACompactTable(t *testing.T) {
	fake := answering(t)
	fake.status = shopStatus()
	t.Setenv("NO_COLOR", "1")

	stdout := &strings.Builder{}

	if code := Run([]string{"status", "--no-json"}, stdout, &strings.Builder{}); code != exitOK {
		t.Fatalf("status = %d", code)
	}

	want := "shop  partial  mode developer  xdebug off  anonymised 2026-11-25 10:00\n" +
		"\n" +
		"  SERVICE         STATE        LAST ERROR\n" +
		"  db              running      -\n" +
		"  nginx           running      2026/11/25 [error] upstream timed out\n" +
		"  search          exited       -\n" +
		"\n" +
		"  URL             ANSWERS      ADDRESS\n" +
		"  base            200          https://shop.local/\n" +
		"  mail            no           http://localhost:8025 (connection refused)\n"

	if diff := cmp.Diff(want, stdout.String()); diff != "" {
		t.Fatalf("text (-want +got):\n%s", diff)
	}
}

// TestStatusSaysWhenTheDataMayBeReal: unknown is never treated as safe, here as everywhere else.
func TestStatusSaysWhenTheDataMayBeReal(t *testing.T) {
	fake := answering(t)
	fake.status = shopStatus()
	fake.status.Anonymised, fake.status.AnonymisedAt = "unknown", ""
	t.Setenv("NO_COLOR", "1")

	stdout := &strings.Builder{}
	Run([]string{"status", "--no-json"}, stdout, &strings.Builder{})

	if first := strings.SplitN(stdout.String(), "\n", 2)[0]; !strings.HasSuffix(first, "data not anonymised") {
		t.Fatalf("first line = %q, want it to say the data is not anonymised", first)
	}
}

func TestStatusRefusesAnOption(t *testing.T) {
	fake := answering(t)
	t.Setenv("COMMAND_BIN_NAME", "")
	stderr := &strings.Builder{}

	if code := Run([]string{"status", "--tonteria", "--json"}, &strings.Builder{}, stderr); code != exitUsage {
		t.Fatalf("status --tonteria = %d, want exitUsage", code)
	}

	if !strings.Contains(stderr.String(), `"hint": "hm status"`) && !strings.Contains(stderr.String(), `"hint":"hm status"`) {
		t.Errorf("stderr = %q, want the usage as the hint", stderr.String())
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked %v, want nothing asked", fake.calls)
	}
}

func TestStatusPassesOnWhatTheEngineRefuses(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "project_not_configured", Code: 4, Message: "not a project", Hint: "hm setup"}}}

	stderr := &strings.Builder{}

	if code := Run([]string{"status", "--json"}, &strings.Builder{}, stderr); code != 4 {
		t.Fatalf("status = %d, want the refusal's code", code)
	}

	if !strings.Contains(stderr.String(), "project_not_configured") {
		t.Errorf("stderr = %q, want the refusal", stderr.String())
	}

	fake.calls = nil
	fake.outcomes = []outcome{{err: errors.New("cannot connect to the Docker daemon")}}

	if code := Run([]string{"status", "--json"}, &strings.Builder{}, &strings.Builder{}); code == exitOK {
		t.Fatal("status = exitOK for an engine that failed")
	}
}
