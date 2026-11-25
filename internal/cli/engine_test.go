package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// What a command that needs a project answers when there is none, through config-env, which asks
// for nothing before it resolves: a refusal from the resolver keeps its own reason, and anything
// else is still the one generic answer.
func TestWhatACommandAnswersWithoutAProject(t *testing.T) {
	noName := core.Refusal{Kind: "no_project_name", Code: 4,
		Message: "No project name can be derived from ___",
		Hint:    "Rename the directory, or set COMPOSE_PROJECT_NAME in config/docker/properties.json"}
	notConfigured := "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid"

	cases := []struct {
		name                string
		err                 error
		kind, message, hint string
	}{
		{"a directory that leaves no name", noName, noName.Kind, noName.Message, noName.Hint},
		{"an error resolving it", errors.New("git: broken"), "project_not_configured", notConfigured, "hm setup"},
		{"a directory that is no project", nil, "project_not_configured", notConfigured, "hm setup"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.outcomes = []outcome{{err: tt.err}}
			t.Setenv("COMMAND_BIN_NAME", "hm")

			var stdout, stderr strings.Builder

			if code := Run([]string{"--json", "config-env"}, &stdout, &stderr); code != exitProject {
				t.Fatalf("config-env = %d, want exitProject", code)
			}

			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}

			assertError(t, stderr.String(), tt.kind, tt.message, tt.hint)

			if len(fake.calls) != 1 {
				t.Errorf("asked the engine %v, want only Resolve", fake.calls)
			}
		})
	}
}
