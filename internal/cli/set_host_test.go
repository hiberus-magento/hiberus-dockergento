package cli

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What set-host asks of the engine. Its two paths fail differently and are exercised
// separately: pointing a domain here resolves a project first, removing one never does.
//

func TestWhatSetHostAsks(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		database bool
	}{
		{"the default asks for the database write", []string{"shop.test"}, true},
		{"--no-database turns the database write off", []string{"shop.test", "--no-database"}, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			code := setHost(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("set-host %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "SetHost", Dir: cwd, Domain: "shop.test", Database: tt.database},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("--json answers the domain and the database flag", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: "/code/shop"}

		stdout := &strings.Builder{}

		code := setHost([]string{"shop.test"}, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("set-host --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		want := map[string]any{"domain": "shop.test", "database": true}

		if diff := cmp.Diff(want, envelope.Data); diff != "" {
			t.Fatalf("set-host --json data (-want +got):\n%s", diff)
		}
	})
}

// TestRemovingAHostAsksNothingAboutTheProject is what proves wrappers.go:280-290 skips
// projectOr for --remove: fake.project stays at its zero value throughout, which a routed
// projectOr would have turned into exitProject, and the log carries no Resolve call at all.
func TestRemovingAHostAsksNothingAboutTheProject(t *testing.T) {
	t.Run("a domain", func(t *testing.T) {
		fake := answering(t)

		code := setHost([]string{"--remove", "shop.test"}, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("set-host --remove shop.test = %d, want exitOK", code)
		}

		want := []call{{Method: "RemoveHost", Domain: "shop.test"}}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("--json answers the domain removed", func(t *testing.T) {
		answering(t)

		stdout := &strings.Builder{}

		code := setHost([]string{"--remove", "shop.test"}, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("set-host --remove --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		want := map[string]any{"removed": "shop.test"}

		if diff := cmp.Diff(want, envelope.Data); diff != "" {
			t.Fatalf("set-host --json data (-want +got):\n%s", diff)
		}
	})

	// --remove alone forwards an empty domain: there is no CLI-level guard against it, left to
	// app/hosts.go:110-118 to refuse. Pinned here, not fixed.
	t.Run("--remove with no domain forwards an empty one", func(t *testing.T) {
		fake := answering(t)

		code := setHost([]string{"--remove"}, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("set-host --remove = %d, want exitOK (the fake never refuses)", code)
		}

		want := []call{{Method: "RemoveHost", Domain: ""}}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})
}

// TestARefusedHostEditIsReported is this change's only exercise of report()'s refusal branch:
// a core.Refusal keeps its own exit code, message and hint, rather than falling into the
// generic exitDocker every other refused call in the package lands on.
func TestARefusedHostEditIsReported(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: core.Refusal{
		Kind:    "no_domain",
		Code:    2,
		Message: "There is no domain to remove",
		Hint:    "hm set-host --remove shop.test",
	}}}

	stderr := &strings.Builder{}

	code := setHost([]string{"shop.test"}, io.Discard, stderr, true)

	if code != 2 {
		t.Fatalf("set-host refused = %d, want 2 (the refusal's own code, not exitDocker)", code)
	}

	var envelope struct {
		Error struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
	}

	want := struct {
		Code    int    `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	}{Code: 2, Type: "no_domain", Message: "There is no domain to remove", Hint: "hm set-host --remove shop.test"}

	if diff := cmp.Diff(want, envelope.Error); diff != "" {
		t.Fatalf("set-host refusal (-want +got):\n%s", diff)
	}
}

func TestASetHostOptionNobodyDeclaredIsAUsageError(t *testing.T) {
	fake := answering(t)

	code := setHost([]string{"-x"}, io.Discard, io.Discard, false)

	if code != exitUsage {
		t.Fatalf("set-host -x = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Fatalf("set-host -x asked the engine %v, want nothing asked at all", fake.calls)
	}
}
