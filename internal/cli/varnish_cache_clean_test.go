package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// The cache:clean varnish-off ends with: through `hm magento` in text mode, exactly as it ran
// before --json was taken care of, and kept off the envelope's stream under --json — where a
// failure is the command's own failure envelope, not a bare exit status.
//

// cacheClean is the cache:clean Exec among the calls, or nil.
func cacheClean(calls []call) *call {
	for at := range calls {
		if calls[at].Method == "Exec" && strings.Contains(strings.Join(calls[at].Command, " "), "cache:clean") {
			return &calls[at]
		}
	}

	return nil
}

func TestVarnishOffInTextModeCleansTheCacheThroughTheMagentoCommand(t *testing.T) {
	t.Run("it runs what hm magento cache:clean runs, with its terminal options", func(t *testing.T) {
		fake := shopForEnvelopes(t)

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"varnish-off", "--no-json"}, &stdout, &stderr); code != exitOK {
			t.Fatalf("hm varnish-off = %d, want exitOK\nstderr: %s", code, stderr.String())
		}

		clean := cacheClean(fake.calls)
		if clean == nil {
			t.Fatal("cache:clean was not run")
		}

		want := call{Method: "Exec", Dir: "/code/shop", Service: phpService,
			Command: []string{"php", "./bin/magento", "cache:clean"}, Options: terminalOptions("")}
		if diff := cmp.Diff(want, *clean); diff != "" {
			t.Fatalf("cache:clean (-want +got):\n%s", diff)
		}

		if !strings.Contains(stdout.String(), "Varnish cache disabled!") || strings.Contains(stdout.String(), "schema_version") {
			t.Fatalf("stdout = %q, want the sentence and no document", stdout.String())
		}
	})

	t.Run("a cache:clean that fails ends it with its own status and nothing more", func(t *testing.T) {
		fake := shopForEnvelopes(t)
		fake.byCommand = func(command []string) (outcome, bool) {
			if strings.Join(command, " ") == "php ./bin/magento cache:clean" {
				return outcome{status: 2}, true
			}

			return outcome{}, false
		}

		var stdout, stderr bytes.Buffer
		if code := Run([]string{"varnish-off", "--no-json"}, &stdout, &stderr); code != 2 {
			t.Fatalf("hm varnish-off = %d, want cache:clean's own 2", code)
		}

		if strings.Contains(stdout.String(), "disabled") || strings.Contains(stderr.String(), "schema_version") {
			t.Fatalf("stdout %q, stderr %q; want no sentence and no document", stdout.String(), stderr.String())
		}
	})
}

func TestVarnishOffUnderJSONReportsAFailedCacheCleanInItsEnvelope(t *testing.T) {
	fake := shopForEnvelopes(t)
	fake.byCommand = func(command []string) (outcome, bool) {
		if strings.Contains(strings.Join(command, " "), "cache:clean") {
			return outcome{status: 2}, true
		}

		return outcome{}, false
	}

	var stdout, stderr bytes.Buffer

	code := Run([]string{"varnish-off", "--json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("hm varnish-off --json = %d, want cache:clean's own 2", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing: there is no success to report", stdout.String())
	}

	var envelope struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   struct {
			Kind    string `json:"type"`
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not one failure envelope (%v):\n%s", err, stderr.String())
	}

	if envelope.OK || envelope.Command != "varnish-off" || envelope.Error.Kind != "command_failed" ||
		envelope.Error.Code != 2 || !strings.Contains(envelope.Error.Message, "cache:clean") {
		t.Fatalf("envelope = %+v, want varnish-off's failure naming cache:clean, with its status", envelope)
	}
}
