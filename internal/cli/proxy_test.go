package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

//
// A proxy subcommand nobody knows is a usage error the binary answers, with the shell
// implementation's type, message and hint. Up, down and status were already the binary's; this was
// the last thing proxy.sh did, and answering(t) pins the fallthrough to a directory with no bin/run,
// so a call that still reached it would fail with 3 here.
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
		"hint":    {envelope.Error.Hint, "hm proxy up | down | status"},
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
