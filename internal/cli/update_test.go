package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func TestUpdateAsksTheEngineAndSaysSo(t *testing.T) {
	fake := answering(t)

	var stdout bytes.Buffer
	if code := update(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("update = %d, want exitOK", code)
	}

	if len(fake.calls) != 1 || fake.calls[0].Method != "Update" {
		t.Fatalf("asked the engine %v, want one Update", fake.calls)
	}

	if !strings.Contains(stdout.String(), "updated!") {
		t.Errorf("stdout = %q, want it to say it was updated", stdout.String())
	}
}

// TestUpdateRefusesAnArgument: it takes none, and one it would ignore is refused.
func TestUpdateRefusesAnArgument(t *testing.T) {
	fake := answering(t)

	if code := update([]string{"--force"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("update --force = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestAnUpdateRefusalKeepsItsCode: a pinned installation is refused with 6.
func TestAnUpdateRefusalKeepsItsCode(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "detached_installation", Code: 6, Message: "pinned"}}}

	if code := update(nil, io.Discard, io.Discard, true); code != 6 {
		t.Fatalf("update of a pinned installation = %d, want 6", code)
	}
}
