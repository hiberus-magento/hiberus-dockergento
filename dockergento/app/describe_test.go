package app

import (
	"errors"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// What Describe answers when there is nothing to describe: a directory whose configuration cannot
// be read is refused as not configured, with the reason and the binary the hint names; a daemon
// that cannot be reached is not a refusal, and stays the error it is.
func TestWhatDescribeAnswersWithNothingToDescribe(t *testing.T) {
	notConfigured := "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid"
	unreachable := errors.New("cannot connect to the Docker daemon")

	cases := []struct {
		name    string
		project core.Project
		config  error
		engine  error
		want    error
	}{
		{"a configuration that cannot be read", core.Project{Name: "shop", Root: "/code/shop"},
			errors.New("no compose file to read in /code/shop"), nil,
			core.Refusal{Kind: "project_not_configured", Code: 4,
				Message: notConfigured + ": no compose file to read in /code/shop", Hint: "hd setup"}},
		// Whoever asks decides what a nameless description means: agent-context answers no_project
		{"a nameless directory whose configuration reads", core.Project{Root: "/code/anywhere"}, nil, nil,
			nil},
		{"a daemon that cannot be reached", core.Project{Name: "shop", Root: "/code/shop"}, nil, unreachable,
			unreachable},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			describer := Describer{
				Engine:  &engine{err: tt.engine},
				Compose: compose{err: tt.config},
				Magento: magento{},
				Tooling: tools{},
				State:   state{},
				Binary:  "hd",
			}

			_, err := describer.Describe(tt.project, core.ComposeFiles{}, false)

			if err != tt.want {
				t.Fatalf("Describe = %#v, want %#v", err, tt.want)
			}
		})
	}
}
