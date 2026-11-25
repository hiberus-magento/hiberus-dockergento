package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// `hm _registry` was scaffolding with a condition for disappearing: nothing in shell left to read
// the registry. With the shell entry point gone it has gone too, and the name is not found like
// any other unknown one — the branch environments are `hm worktree list`'s to show.
func TestTheRegistryDiagnosticIsGone(t *testing.T) {
	fake := answering(t)

	var stderr bytes.Buffer

	if code := Run([]string{"--json", "_registry"}, io.Discard, &stderr); code != exitUsage {
		t.Fatalf("hm _registry = %d, want exitUsage; stderr = %q", code, stderr.String())
	}

	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || envelope.Error.Type != "command_not_found" {
		t.Fatalf("stderr = %q, want a command_not_found envelope", stderr.String())
	}

	if len(fake.calls) != 0 {
		t.Fatalf("asked of the engine %v, want nothing", fake.calls)
	}
}
