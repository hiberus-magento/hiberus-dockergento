package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

//
// A proxy subcommand nobody knows is a usage error the binary answers, with the shell
// implementation's type, message and hint. Up, down and status were already the binary's; this was
// the last thing proxy.sh did.
//

func TestAnUnknownProxySubcommandIsAUsageError(t *testing.T) {
	fake := answering(t)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--json", "proxy", "sideways"}, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("hm proxy sideways = %d, want exitUsage\nstderr: %s", code, stderr.String())
	}

	var envelope struct {
		Command string `json:"command"`
		Error   struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	want := map[string][2]string{
		"command": {envelope.Command, "proxy"},
		"type":    {envelope.Error.Type, "unknown_subcommand"},
		"message": {envelope.Error.Message, "'sideways' is not something hm proxy does"},
		"hint":    {envelope.Error.Hint, "hm proxy up | down | status | certify <domain>"},
	}
	for field, pair := range want {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q, want %q", field, pair[0], pair[1])
		}
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// `proxy certify <domain>` is the step setup runs for a project on the proxy, on its own: what lets
// a suite prove against a real Traefik that hm restarts the proxy itself after a new certificate.
// A domain that is not one is refused before anything is touched, being a file name too.
func TestProxyCertifyRefusesWhatIsNotADomain(t *testing.T) {
	for _, args := range [][]string{{"proxy", "certify"}, {"proxy", "certify", "../etc"},
		{"proxy", "certify", "a.local", "b.local"}, {"proxy", "certify", "-x.local"}} {
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--json"}, args...), &stdout, &stderr)

		var envelope struct {
			Error struct {
				Hint string `json:"hint"`
			} `json:"error"`
		}
		_ = json.Unmarshal(stderr.Bytes(), &envelope)

		if code != exitUsage || envelope.Error.Hint != "hm proxy certify <domain>" {
			t.Errorf("%v = %d, %s; want a usage error naming the form", args, code, stderr.String())
		}
	}
}

// Without mkcert nothing is signed and the proxy is not touched, and asked for on its own that is
// a refusal: answering {"certified": …} with 0 would claim a certificate nobody signed.
func TestProxyCertifyWithoutMkcertIsRefused(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HM_PROXY_DIR", t.TempDir())
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--json", "proxy", "certify", "shop.local"}, &stdout, &stderr)

	var envelope struct {
		Error struct {
			Type string `json:"type"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	_ = json.Unmarshal(stderr.Bytes(), &envelope)

	if code != exitError || envelope.Error.Type != "mkcert_missing" || stdout.Len() != 0 {
		t.Fatalf("= %d\nstdout %s\nstderr %s", code, stdout.String(), stderr.String())
	}

	if !strings.Contains(envelope.Error.Hint, "mkcert") {
		t.Errorf("hint = %q, want it to say how to get mkcert", envelope.Error.Hint)
	}

	var text bytes.Buffer
	if code := Run([]string{"--no-json", "proxy", "certify", "shop.local"}, &bytes.Buffer{}, &text); code != exitError ||
		!strings.Contains(text.String(), "mkcert is not installed") {
		t.Errorf("text mode = %d, %q", code, text.String())
	}
}
