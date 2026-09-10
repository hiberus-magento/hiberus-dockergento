package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

//
// What describe refuses before it ever reaches an engine. No fake, no answering(t): the refusal
// returns during parsing, before any engine call is made — the same shape down_test.go:11-18
// established. Both rows pin COMMAND_BIN_NAME explicitly, the version_test.go:86 precedent:
// bin/run:4 exports COMMAND_BIN_NAME="hm", so an ambient value would otherwise decide the default
// row by accident, and the renamed-binary row is what can only pass once binaryName()
// (engine.go:106-112) is genuinely called.
//

func TestDescribeRefusesAnUnknownOption(t *testing.T) {
	cases := []struct {
		name       string
		binaryName string
		args       []string
		wantHint   string
	}{
		{"the circular --help hint", "hm", []string{"--help"}, "hm describe [--with-secrets]"},
		{"a renamed binary", "hd", []string{"--tonteria"}, "hd describe [--with-secrets]"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("COMMAND_BIN_NAME", tt.binaryName)

			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := describe(tt.args, stdout, stderr, true)

			if code != exitUsage {
				t.Fatalf("describe %v = %d, want exitUsage", tt.args, code)
			}

			var envelope struct {
				Error struct {
					Hint string `json:"hint"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Hint != tt.wantHint {
				t.Fatalf("describe %v hint = %q, want %q", tt.args, envelope.Error.Hint, tt.wantHint)
			}
		})
	}
}
