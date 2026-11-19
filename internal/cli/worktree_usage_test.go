package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

//
// worktree with no subcommand, or one it does not know, is answered by the binary as a usage error.
//
// It used to fall through to the shell entry point, whose worktree.sh did nothing but exec the
// binary again with the same arguments: binary, bin/run, worktree.sh, binary, for as long as Docker
// was up for bin/run's validation to pass. answering(t) pins the fallthrough to a directory with no
// bin/run, so a call that still falls through fails with 3 here instead of looping.
//

func TestWorktreeWithoutAKnownSubcommandIsAUsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no subcommand", []string{"--json", "worktree"}},
		{"an unknown subcommand", []string{"--json", "worktree", "nonsense"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)

			if code != exitUsage {
				t.Fatalf("hm %s = %d, want exitUsage\nstderr: %s", strings.Join(tt.args, " "), code, stderr.String())
			}

			var envelope struct {
				Error struct {
					Type string `json:"type"`
					Hint string `json:"hint"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
			}

			if envelope.Error.Type != "invalid_argument" {
				t.Errorf("error type = %q, want invalid_argument", envelope.Error.Type)
			}

			if !strings.Contains(envelope.Error.Hint, "worktree add|list|remove") {
				t.Errorf("hint = %q, want it to name the three subcommands", envelope.Error.Hint)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}

			if len(fake.calls) != 0 {
				t.Errorf("asked the engine %v, want nothing", fake.calls)
			}
		})
	}
}
