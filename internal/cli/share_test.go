package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// share's command line: which service it tunnels to, the question before exposing anything, what
// it prints, and that the tunnel is held and then closed. What it asks of Docker is the use case's,
// held in dockergento/app/share_test.go.
//

func sharingShop(t *testing.T) *fakeEngine {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Domain: "shop.test"}
	t.Setenv("NO_COLOR", "1")

	return fake
}

func TestShareTunnelsToVarnishHoldsItAndClosesIt(t *testing.T) {
	t.Setenv("HM_NON_INTERACTIVE", "1")

	fake := sharingShop(t)
	fake.share = core.Share{URL: "https://a-b.trycloudflare.com", Project: "shop", Relay: "hm-share-shop"}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--json", "share"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("share = %d\nstderr: %s", code, stderr.String())
	}

	var envelope struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}

	if diff := cmp.Diff(map[string]string{"url": "https://a-b.trycloudflare.com", "project": "shop"}, envelope.Data); diff != "" {
		t.Errorf("data (-want +got):\n%s", diff)
	}

	// HM_NON_INTERACTIVE is set by answering(t): nobody to ask, so it goes ahead
	want := []string{"Resolve", "Share varnish 6081 true", "HoldShare hm-share-shop", "Unshare"}
	if diff := cmp.Diff(want, shareCalls(fake)); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}

	// What closing says goes to stderr under --json: stdout is the document
	if !strings.Contains(stderr.String(), "Sharing stopped.") {
		t.Errorf("stderr = %q, want it to say sharing stopped", stderr.String())
	}
}

func TestShareTellsWhereAndHowLongInText(t *testing.T) {
	fake := sharingShop(t)
	fake.share = core.Share{URL: "https://a-b.trycloudflare.com", Project: "shop", Relay: "hm-share-shop"}

	var stdout bytes.Buffer
	Run([]string{"--no-json", "share"}, &stdout, &bytes.Buffer{})

	for _, line := range []string{
		"'shop' is reachable at https://a-b.trycloudflare.com\n",
		"  The address changes every time, and it is gone when you stop.\n",
		"  Leave this running while you use it. Ctrl-C stops sharing.\n",
		"Sharing stopped.\n",
	} {
		if !strings.Contains(stdout.String(), line) {
			t.Errorf("stdout lacks %q:\n%s", line, stdout.String())
		}
	}
}

func TestShareTakesTheServiceAndPortFromTheEnvironment(t *testing.T) {
	fake := sharingShop(t)
	fake.share = core.Share{URL: "https://a-b.trycloudflare.com", Project: "shop", Relay: "hm-share-shop"}
	t.Setenv("HM_SHARE_SERVICE", "nginx")
	t.Setenv("HM_SHARE_PORT", "8000")

	Run([]string{"--json", "share"}, &bytes.Buffer{}, &bytes.Buffer{})

	if got := shareCalls(fake); len(got) < 2 || got[1] != "Share nginx 8000 true" {
		t.Errorf("calls = %v, want nginx on 8000", got)
	}
}

func TestShareAsksWhenSomebodyCanAnswer(t *testing.T) {
	fake := sharingShop(t)
	t.Setenv("HM_NON_INTERACTIVE", "")

	var stdout bytes.Buffer
	if code := Run([]string{"--no-json", "share"}, &stdout, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("a declined share = %d, want 0", code)
	}

	// What is exposed is named; the test's input has no answer, which is a no
	if !strings.Contains(stdout.String(), "This puts 'shop' on the public internet.") {
		t.Errorf("stdout = %q, want it to name what would be exposed", stdout.String())
	}

	// A no holds nothing open and closes nothing
	want := []string{"Resolve", "Share varnish 6081 false"}
	if diff := cmp.Diff(want, shareCalls(fake)); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

func TestShareStopClosesTheTunnel(t *testing.T) {
	for _, tt := range []struct {
		present bool
		want    string
	}{
		{true, "The tunnel is closed.\n"},
		{false, "Nothing was being shared.\n"},
	} {
		fake := sharingShop(t)
		fake.unshared = tt.present

		var stdout bytes.Buffer
		if code := Run([]string{"--no-json", "share", "--stop"}, &stdout, &bytes.Buffer{}); code != exitOK {
			t.Fatalf("share --stop = %d", code)
		}

		if stdout.String() != tt.want {
			t.Errorf("with a tunnel=%v stdout = %q, want %q", tt.present, stdout.String(), tt.want)
		}

		if diff := cmp.Diff([]string{"Resolve", "Unshare"}, shareCalls(fake)); diff != "" {
			t.Errorf("calls (-want +got):\n%s", diff)
		}

		var document bytes.Buffer
		Run([]string{"--json", "share", "--stop"}, &document, &bytes.Buffer{})

		if !strings.Contains(document.String(), `"closed": `+map[bool]string{true: "true", false: "false"}[tt.present]) {
			t.Errorf("--json says %s", document.String())
		}
	}
}

func TestShareRefusesAnOptionItDoesNotKnow(t *testing.T) {
	fake := sharingShop(t)

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "share", "--public"}, &bytes.Buffer{}, &stderr); code != exitUsage {
		t.Fatalf("share --public = %d, want exitUsage", code)
	}

	for _, part := range []string{`"invalid_argument"`, "Unknown option: --public", "hm share [--stop]"} {
		if !strings.Contains(stderr.String(), part) {
			t.Errorf("stderr lacks %s: %s", part, stderr.String())
		}
	}

	if diff := cmp.Diff([]string{"Resolve"}, shareCalls(fake)); diff != "" {
		t.Errorf("calls (-want +got):\n%s", diff)
	}
}

func shareCalls(fake *fakeEngine) []string {
	described := make([]string, 0, len(fake.calls))

	for _, one := range fake.calls {
		switch one.Method {
		case "Share":
			described = append(described, strings.Join([]string{"Share", one.Service,
				strconv.Itoa(one.Port), map[bool]string{true: "true", false: "false"}[one.Confirm]}, " "))
		case "HoldShare":
			described = append(described, "HoldShare "+one.Key)
		default:
			described = append(described, one.Method)
		}
	}

	return described
}
