package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// The permissions an agent using the tool is given: each command allowed, or asked about, by how
// safe command_descriptions.json says it is, and the paths it must never read denied — what
// permissions.sh built with jq, now from the same two files.
//

const someCommands = `{
  "_comment": {"safety": "safe"},
  "_order": ["describe", "start", "down"],
  "describe": {"safety": "safe"},
  "start": {"safety": "caution"},
  "down": {"safety": "dangerous"}
}`

const someExclusions = `{"exclusions": [{"path": "var/log"}, {"path": "app/etc/env.php"}]}`

func TestAgentPermissionsByHowSafeEachCommandIs(t *testing.T) {
	got, err := ReadAgentPermissions([]byte(someCommands), []byte(someExclusions), "hm", false)
	if err != nil {
		t.Fatalf("ReadAgentPermissions = %v", err)
	}

	want := AgentPermissions{}
	want.Permissions.Allow = []string{"Bash(hm describe:*)", "Bash(hm start:*)"}
	want.Permissions.Ask = []string{"Bash(hm down:*)"}
	want.Permissions.Deny = []string{"Read(./app/etc/env.php)", "Read(./var/log/**)"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("permissions (-want +got):\n%s", diff)
	}
}

// TestStrictAllowsOnlyWhatHasNoSideEffects: caution moves to the questions.
func TestStrictAllowsOnlyWhatHasNoSideEffects(t *testing.T) {
	got, err := ReadAgentPermissions([]byte(someCommands), []byte(someExclusions), "hm", true)
	if err != nil {
		t.Fatalf("ReadAgentPermissions = %v", err)
	}

	if diff := cmp.Diff([]string{"Bash(hm describe:*)"}, got.Permissions.Allow); diff != "" {
		t.Errorf("allow (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"Bash(hm down:*)", "Bash(hm start:*)"}, got.Permissions.Ask); diff != "" {
		t.Errorf("ask (-want +got):\n%s", diff)
	}
}

// TestTheBinaryNameIsTheOneInvoked: an installation under another name gets rules for that name.
func TestTheBinaryNameIsTheOneInvoked(t *testing.T) {
	got, err := ReadAgentPermissions([]byte(someCommands), []byte(someExclusions), "dockergento", false)
	if err != nil {
		t.Fatalf("ReadAgentPermissions = %v", err)
	}

	if got.Permissions.Allow[0] != "Bash(dockergento describe:*)" {
		t.Errorf("allow[0] = %q, want the invoked name", got.Permissions.Allow[0])
	}
}

func TestMalformedDataIsAnError(t *testing.T) {
	if _, err := ReadAgentPermissions([]byte("{"), []byte(someExclusions), "hm", false); err == nil {
		t.Error("malformed command descriptions = no error, want one")
	}

	if _, err := ReadAgentPermissions([]byte(someCommands), []byte("{"), "hm", false); err == nil {
		t.Error("malformed exclusions = no error, want one")
	}
}
