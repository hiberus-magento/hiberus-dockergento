package cli

import (
	"bytes"
	"encoding/json"
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

// Without mkcert nothing is signed and the proxy is not touched, as in setup, and it says why.
func TestProxyCertifyWithoutMkcertSaysSo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HM_PROXY_DIR", t.TempDir())
	t.Setenv("NO_COLOR", "1")

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--no-json", "proxy", "certify", "shop.local"}, &stdout, &stderr)

	if code != exitOK || !bytes.Contains(append(stdout.Bytes(), stderr.Bytes()...), []byte("mkcert is not installed")) {
		t.Fatalf("= %d\nstdout %s\nstderr %s", code, stdout.String(), stderr.String())
	}
}
