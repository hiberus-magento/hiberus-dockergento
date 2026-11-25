package app

import (
	"errors"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The deploy mode is env.php's MAGE_MODE, and a Magento whose env.php does not set it runs in
// "default" — Magento's own State answers that — so that is what is said, not "unknown". Only
// where there is no Magento to be in a mode is the mode left unsaid.
func TestTheModeOfAMagentoThatSetsNoneIsDefault(t *testing.T) {
	cases := []struct {
		name             string
		version, written string
		want             string
	}{
		{"the mode env.php sets", "2.4.8-p3", "production", "production"},
		{"no MAGE_MODE in env.php", "2.4.8-p3", "", "default"},
		{"no Magento found at all", "", "", ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			describer := Describer{
				Engine:  &engine{},
				Compose: compose{},
				Magento: magento{version: tt.version, mode: tt.written},
				Tooling: tools{},
				State:   state{},
			}

			description, err := describer.Describe(core.Project{Name: "shop", Root: "/code/shop"}, core.ComposeFiles{}, false)
			if err != nil {
				t.Fatalf("Describe = %v", err)
			}

			if description.Magento.Mode != tt.want {
				t.Fatalf("mode = %q, want %q", description.Magento.Mode, tt.want)
			}
		})
	}
}

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
		// Whoever asks decides what a nameless description means: agent-context answers it with
		// its own no_project refusal, so describe does not refuse it here
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
