package cli

import (
	"bytes"
	"errors"
	"testing"
)

//
// compatibility prints the table the engine builds (core/compatibility_test.go holds the table),
// as it is and wherever it is run: it needs no project, and like the shell it prints the table
// under --json too.
//

func TestCompatibilityPrintsTheTableOutsideAProject(t *testing.T) {
	fake := answering(t)
	fake.compatibility = "\n===\n| table |\n===\n"
	t.Setenv("NO_COLOR", "1")

	for _, args := range [][]string{{"--no-json", "compatibility"}, {"--json", "compatibility"}} {
		var stdout bytes.Buffer
		if code := Run(args, &stdout, &bytes.Buffer{}); code != exitOK {
			t.Fatalf("%v = %d", args, code)
		}

		if stdout.String() != fake.compatibility {
			t.Errorf("%v printed %q, want the table as it is", args, stdout.String())
		}
	}
}

func TestCompatibilityWithoutItsDataSaysSo(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: errors.New("open data/requirements.json: no such file")}}

	var stderr bytes.Buffer
	if code := Run([]string{"--json", "compatibility"}, &bytes.Buffer{}, &stderr); code == exitOK {
		t.Fatalf("compatibility without its data = 0, want a failure\nstderr: %s", stderr.String())
	}
}
