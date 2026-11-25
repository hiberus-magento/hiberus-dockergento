package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The commands that change something and print nothing of their own answer --json with one success
// envelope on stdout, and nothing else there. Text mode stays what it was: these print nothing on
// stdout, the engine's announcements go to stderr.
//

// succeededEnvelope is what a command printed on stdout, decoded. It fails unless stdout holds
// exactly one JSON document and nothing after it.
type succeededEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	Command       string         `json:"command"`
	OK            bool           `json:"ok"`
	Data          map[string]any `json:"data"`
}

func succeeded(t *testing.T, args ...string) (succeededEnvelope, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr); code != exitOK {
		t.Fatalf("hm %v = %d, want exitOK\nstderr: %s", args, code, stderr.String())
	}

	decoder := json.NewDecoder(&stdout)

	var envelope succeededEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("hm %v: stdout is not a JSON envelope: %v", args, err)
	}

	if rest, _ := io.ReadAll(decoder.Buffered()); strings.TrimSpace(string(rest)) != "" {
		t.Fatalf("hm %v: stdout carries more than the envelope: %q", args, rest)
	}

	if rest, _ := io.ReadAll(&stdout); strings.TrimSpace(string(rest)) != "" {
		t.Fatalf("hm %v: stdout carries more than the envelope: %q", args, rest)
	}

	if envelope.SchemaVersion != 1 || !envelope.OK {
		t.Fatalf("hm %v: envelope = %+v, want schema_version 1 and ok", args, envelope)
	}

	return envelope, stderr.String()
}

// withoutElapsed checks that elapsed_ms is a non-negative number and takes it out, so that the rest
// of the data can be compared whole.
func withoutElapsed(t *testing.T, data map[string]any) map[string]any {
	t.Helper()

	elapsed, ok := data["elapsed_ms"].(float64)
	if !ok || elapsed < 0 {
		t.Fatalf("elapsed_ms = %#v, want a non-negative number", data["elapsed_ms"])
	}

	delete(data, "elapsed_ms")

	return data
}

func shopForEnvelopes(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop", Domain: "shop.test"}

	return fake
}

func TestSuccessEnvelopes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			"start of the whole environment carries its URL", []string{"start", "--json"},
			map[string]any{
				"project": "shop", "services": []any{}, "whole_environment": true,
				"stopped_others": false, "url": "https://shop.test/",
			},
		},
		{
			"start of named services after stopping the rest", []string{"start", "-s", "phpfpm", "--json"},
			map[string]any{
				"project": "shop", "services": []any{"phpfpm"}, "whole_environment": false,
				"stopped_others": true, "url": "https://shop.test/",
			},
		},
		{
			"stop with a snapshot and a timeout", []string{"stop", "--snapshot", "-t", "5", "db", "--json"},
			map[string]any{
				"project": "shop", "services": []any{"db"}, "whole_environment": false,
				"snapshot": true, "timeout": float64(5),
			},
		},
		{
			"stop with Compose's own timeout", []string{"stop", "--json"},
			map[string]any{
				"project": "shop", "services": []any{}, "whole_environment": true,
				"snapshot": false, "timeout": nil,
			},
		},
		{
			"restart", []string{"restart", "nginx", "--json"},
			map[string]any{
				"project": "shop", "services": []any{"nginx"}, "whole_environment": false, "timeout": nil,
			},
		},
		{
			"rebuild", []string{"rebuild", "--json"},
			map[string]any{"project": "shop", "services": []any{}, "whole_environment": true},
		},
		{
			"post-start", []string{"post-start", "--json"},
			map[string]any{"project": "shop"},
		},
		{
			"purge names what it removed", []string{"purge", "--json"},
			map[string]any{"project": "shop", "removed": []any{
				"var/cache/*", "generated/*", "pub/static/*", "var/view_preprocessed/*",
				"var/page_cache/*", "var/generation/*", "dev/tests/integration/tmp/*",
			}},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			shopForEnvelopes(t)

			envelope, _ := succeeded(t, tt.args...)

			if envelope.Command != tt.args[0] {
				t.Errorf("command = %q, want %q", envelope.Command, tt.args[0])
			}

			if diff := cmp.Diff(tt.want, withoutElapsed(t, envelope.Data)); diff != "" {
				t.Errorf("data (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStartWithoutADomainHasNoURL: the URL is what the project resolved to, never a guess.
func TestStartWithoutADomainHasNoURL(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}

	envelope, _ := succeeded(t, "start", "--json")

	if url, present := envelope.Data["url"]; !present || url != nil {
		t.Fatalf("url = %#v (present %v), want null", url, present)
	}
}

// TestDownDeclinedIsAnEnvelopeToo: when the question is answered "nothing", --json says so in the
// document instead of a sentence on stdout.
func TestDownDeclinedIsAnEnvelopeToo(t *testing.T) {
	shopForEnvelopes(t).outcomes = []outcome{{}, {captured: ""}}

	envelope, _ := succeeded(t, "down", "-v", "--json")

	want := map[string]any{"destroyed": false, "volumes": true, "snapshot": false}
	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Fatalf("data (-want +got):\n%s", diff)
	}
}

// TestDownDestroyedKeepsItsEnvelope: the envelope down already printed is unchanged.
func TestDownDestroyedKeepsItsEnvelope(t *testing.T) {
	shopForEnvelopes(t).outcomes = []outcome{{}, {captured: "saved"}}

	envelope, _ := succeeded(t, "down", "-v", "--json")

	want := map[string]any{"destroyed": true, "volumes": true, "snapshot": true}
	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Fatalf("data (-want +got):\n%s", diff)
	}
}

// TestTextModeIsUnchanged pins what these commands print on stdout without --json: nothing of
// their own, except down's sentence when nothing was destroyed.
func TestTextModeIsUnchanged(t *testing.T) {
	cases := []struct {
		args     []string
		outcomes []outcome
		want     string
	}{
		{[]string{"start", "--no-json"}, nil, ""},
		{[]string{"stop", "--no-json"}, nil, ""},
		{[]string{"restart", "--no-json"}, nil, ""},
		{[]string{"rebuild", "--no-json"}, nil, ""},
		{[]string{"post-start", "--no-json"}, nil, ""},
		{[]string{"purge", "--no-json"}, nil, ""},
		{[]string{"down", "--no-json"}, []outcome{{}, {captured: "destroyed"}}, ""},
		{[]string{"down", "--no-json"}, []outcome{{}, {captured: ""}}, "Nothing was destroyed.\n"},
	}

	for _, tt := range cases {
		t.Run(strings.Join(tt.args, " ")+" "+tt.want, func(t *testing.T) {
			shopForEnvelopes(t).outcomes = tt.outcomes
			t.Setenv("NO_COLOR", "1")

			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != exitOK {
				t.Fatalf("hm %v = %d, want exitOK\nstderr: %s", tt.args, code, stderr.String())
			}

			if stdout.String() != tt.want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

// TestAFailureIsStillOnlyTheErrorEnvelope: a failed operation prints its error envelope on stderr,
// as it did, and no success envelope on stdout.
func TestAFailureIsStillOnlyTheErrorEnvelope(t *testing.T) {
	for _, command := range []string{"start", "stop", "restart", "rebuild", "post-start"} {
		t.Run(command, func(t *testing.T) {
			shopForEnvelopes(t).outcomes = []outcome{{}, {err: errors.New("compose said no")}}

			var stdout, stderr bytes.Buffer
			if code := Run([]string{command, "--json"}, &stdout, &stderr); code != exitDocker {
				t.Fatalf("hm %s = %d, want exitDocker", command, code)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}

			want := "{\n  \"schema_version\": 1,\n  \"command\": \"" + command + "\",\n  \"ok\": false,\n" +
				"  \"error\": {\n    \"code\": 3,\n    \"type\": \"docker_failed\",\n" +
				"    \"message\": \"compose said no\"\n  }\n}\n"
			if stderr.String() != want {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
		})
	}
}

// TestPurgeFailingInTheContainerPrintsNoEnvelope: the container's exit code is returned as it was,
// and stdout holds no success document.
func TestPurgeFailingInTheContainerPrintsNoEnvelope(t *testing.T) {
	shopForEnvelopes(t).outcomes = []outcome{{}, {status: 1}}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"purge", "--json"}, &stdout, &stderr); code != 1 {
		t.Fatalf("hm purge = %d, want 1", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", stdout.String())
	}
}

// TestVarnishOffPrintsOneDocument: varnish-off purges on the way, and purge's own envelope must
// not be printed there — the reader expects one document, varnish-off's.
func TestVarnishOffPurgesWithoutAnEnvelopeOfItsOwn(t *testing.T) {
	shopForEnvelopes(t)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"varnish-off", "--json"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("hm varnish-off = %d, want exitOK\nstderr: %s", code, stderr.String())
	}

	if count := strings.Count(stdout.String(), "\"schema_version\""); count != 1 {
		t.Fatalf("stdout holds %d envelopes, want 1:\n%s", count, stdout.String())
	}

	if !strings.Contains(stdout.String(), "\"command\": \"varnish-off\"") {
		t.Fatalf("stdout = %s, want varnish-off's envelope", stdout.String())
	}
}
