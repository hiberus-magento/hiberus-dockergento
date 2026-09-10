package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

//
// doctor otherwise has no place in this cycle — it is about list, describe and logs — but its
// unknown-option hint is character-for-character the same defect the other three carry: a
// hardcoded "hm doctor --help" that both names a binary nobody renamed to and points straight back
// at the refusal that produced it. Fixing three of the four and leaving this one would read like a
// deliberate exception to a convention it simply violates, so it comes along. This is doctor's
// only Go test in this cycle: no fake, no answering(t), the refusal returns during parsing, the
// same shape down_test.go:11-18 established. Both rows pin COMMAND_BIN_NAME explicitly, the
// version_test.go:86 precedent: bin/run:4 exports COMMAND_BIN_NAME="hm", so an ambient value would
// otherwise decide the default row by accident, and the renamed-binary row is what can only pass
// once binaryName() (engine.go:106-112) is genuinely called.
//

func TestDoctorRefusesAnUnknownOption(t *testing.T) {
	cases := []struct {
		name       string
		binaryName string
		args       []string
		wantHint   string
	}{
		{"the circular --help hint", "hm", []string{"--help"}, "hm doctor [--only=ports]"},
		{"a renamed binary", "hd", []string{"--tonteria"}, "hd doctor [--only=ports]"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("COMMAND_BIN_NAME", tt.binaryName)

			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := doctor(tt.args, stdout, stderr, true)

			if code != exitUsage {
				t.Fatalf("doctor %v = %d, want exitUsage", tt.args, code)
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
				t.Fatalf("doctor %v hint = %q, want %q", tt.args, envelope.Error.Hint, tt.wantHint)
			}
		})
	}
}
