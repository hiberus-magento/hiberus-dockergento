package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func agentPermissions() core.AgentPermissions {
	p := core.AgentPermissions{}
	p.Permissions.Allow = []string{"Bash(hm describe:*)"}
	p.Permissions.Ask = []string{"Bash(hm down:*)"}
	p.Permissions.Deny = []string{"Read(./var/log/**)"}

	return p
}

// TestPermissionsUnderJSONIsTheConfigurationItself: what gets pasted into an agent's settings,
// with no envelope around it, as the shell printed it.
func TestPermissionsUnderJSONIsTheConfigurationItself(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()

	var stdout bytes.Buffer
	if code := permissions(nil, &stdout, io.Discard, true); code != exitOK {
		t.Fatalf("permissions --json = %d, want exitOK", code)
	}

	var got core.AgentPermissions
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not the configuration: %v\n%s", err, stdout.String())
	}

	if got.Permissions.Allow[0] != "Bash(hm describe:*)" || got.Permissions.Deny[0] != "Read(./var/log/**)" {
		t.Errorf("configuration = %+v", got)
	}

	if strings.Contains(stdout.String(), "schema_version") {
		t.Errorf("stdout carries the envelope: %s", stdout.String())
	}
}

func TestPermissionsStrictIsPassedOn(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()

	var stdout bytes.Buffer
	if code := permissions([]string{"--strict"}, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("permissions --strict = %d, want exitOK", code)
	}

	if len(fake.calls) != 1 || !fake.calls[0].All {
		t.Fatalf("asked %v, want one strict AgentPermissions", fake.calls)
	}

	if !strings.Contains(stdout.String(), "Strict: only commands with no side effects") {
		t.Errorf("stdout = %q, want the strict summary", stdout.String())
	}
}

// TestPermissionsInTextSaysHowManyAndWritesNothing.
func TestPermissionsInTextSaysHowManyAndWritesNothing(t *testing.T) {
	fake := answering(t)
	fake.permissions = agentPermissions()

	var stdout bytes.Buffer
	if code := permissions(nil, &stdout, io.Discard, false); code != exitOK {
		t.Fatalf("permissions = %d, want exitOK", code)
	}

	for _, want := range []string{"1 command(s) allowed, 1 asking for confirmation.", "1 path(s) denied outright",
		"Nothing was written for you"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
}

func TestPermissionsRefusesAnOption(t *testing.T) {
	fake := answering(t)

	if code := permissions([]string{"--loose"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("permissions --loose = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked %v, want nothing", fake.calls)
	}
}

// TestTheRouterAnswersForPermissions: it reaches its handler, not the shell.
func TestTheRouterAnswersForPermissions(t *testing.T) {
	fake := answering(t)

	Run([]string{"permissions"}, io.Discard, io.Discard)

	if len(fake.calls) == 0 {
		t.Fatal("permissions fell through to the shell implementation")
	}
}
