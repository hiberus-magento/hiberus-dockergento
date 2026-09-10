package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What describe refuses before it ever reaches an engine. No fake, no answering(t): the refusal
// returns during parsing, before any engine call is made — the same shape down_test.go:11-18
// established. Both rows pin COMMAND_BIN_NAME explicitly, the version_test.go:86 precedent:
// bin/run:4 exports COMMAND_BIN_NAME="hm", so an ambient value would otherwise decide the default
// row by accident, and the renamed-binary row is what can only pass once binaryName()
// (engine.go:106-112) is genuinely called.
//
// RED-window safety, named rather than assumed — this is the one command whose RED can exec into
// a live container, not just read a container list. Resolve does not fail on a directory that is
// not a project (dockergento.go:128-131: "a directory that is not a project is not an error … the
// answer is a project with no name"). So an unrouted describe proceeds into
// app.Describer.Describe's seven concurrent questions (app/describe.go:58-70) before inspecting
// any error, and two of them are d.Engine.Containers() and d.Tooling.Xdebug(project.Name). With a
// nameless project, toolinfo/tooling.go:84-85 matches:
//
//	matches := item.Labels["hm.project"] == project ||
//	    (item.Labels["hm.project"] == "" && item.Labels["com.docker.compose.project"] == project)
//
// — with project == "" the first disjunct is true of every container lacking the hm.project
// label, so combined with the com.docker.compose.service == "phpfpm" test
// (toolinfo/tooling.go:87), Xdebug selects the first unlabelled phpfpm container on the whole
// machine and runs ContainerExecCreate/ContainerExecAttach inside it. This machine has the user's
// rabatrepo project running with 9 live containers, at least one phpfpm among them.
// DOCKER_HOST=unix:///nonexistent (fake_engine_test.go:222) is the only thing between this unit's
// RED run and execing into somebody else's running container. Every subtest below that exercises
// describe runs under answering(t), and none overrides DOCKER_HOST.
//

func TestDescribeRefusesAnUnknownOption(t *testing.T) {
	cases := []struct {
		name       string
		binaryName string
		args       []string
		wantHint   string
		withFake   bool
	}{
		{"the circular --help hint", "hm", []string{"--help"}, "hm describe [--with-secrets]", false},
		{"a renamed binary", "hd", []string{"--tonteria"}, "hd describe [--with-secrets]", false},
		{"a positional argument under a substituted engine", "hm", []string{"algo"}, "hm describe [--with-secrets]", true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var fake *fakeEngine
			if tt.withFake {
				fake = answering(t)
			} else {
				t.Chdir(t.TempDir())
			}
			t.Setenv("COMMAND_BIN_NAME", tt.binaryName)

			stdout, stderr := &strings.Builder{}, &strings.Builder{}

			code := describe(tt.args, stdout, stderr, true)

			if code != exitUsage {
				t.Fatalf("describe %v = %d, want exitUsage", tt.args, code)
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
				t.Fatalf("describe %v hint = %q, want %q", tt.args, envelope.Error.Hint, tt.wantHint)
			}

			// The row under a substituted engine also proves describe.go:15-22 never lets a
			// positional through to it: a future loosening of that guard would show up here as
			// a non-empty call log, not just as a wrong hint.
			if tt.withFake && len(fake.calls) != 0 {
				t.Fatalf("describe %v asked the engine %v, want nothing asked at all", tt.args, fake.calls)
			}
		})
	}
}

// TestWhatDescribeAsks is what describe asks of the engine: exactly one call, about no project at
// all — describe.go never calls projectOr, so f.project plays no part in this command. The
// no-arguments case is the highest-value one in the cycle: WithSecrets defaulting to true would
// volunteer the database password into every log, issue and agent context.
func TestWhatDescribeAsks(t *testing.T) {
	cases := []struct {
		name            string
		args            []string
		wantWithSecrets bool
	}{
		{"no arguments", nil, false},
		{"--with-secrets", []string{"--with-secrets"}, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)

			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("os.Getwd() = %v", err)
			}

			code := describe(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("describe %v = %d, want exitOK", tt.args, code)
			}

			want := []call{{Method: "Describe", Dir: cwd, WithSecrets: tt.wantWithSecrets}}
			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestWhatDescribeReportsAsJSON decodes describe --json against a hand-written literal, never a
// re-marshalled fixture — comparing the encoder with itself would pass through any tag rename.
// The credentials key must be absent, not null, when the engine returned none: an omitempty
// regression would otherwise put a database password's absence in every document as "null"
// instead of leaving the key out.
func TestWhatDescribeReportsAsJSON(t *testing.T) {
	base := map[string]any{
		"project": map[string]any{
			"name":     "shop",
			"root":     "/code/shop",
			"worktree": "feature-x",
			"domain":   "shop.local",
			"status":   "running",
			"urls": map[string]any{
				"base":     "https://shop.local/",
				"admin":    "https://shop.local/admin_1a2b3c",
				"mail":     "http://localhost:8025",
				"mailhog":  "http://localhost:8025",
				"rabbitmq": "",
				"search":   "http://localhost:9200",
			},
		},
		"magento": map[string]any{"version": "2.4.6", "mode": "developer"},
		"data":    map[string]any{"anonymised": "yes", "anonymised_at": "2026-01-01T00:00:00Z"},
		"services": []any{
			map[string]any{
				"name": "phpfpm", "image": "hiberusmagento/phpfpm:8.2", "state": "running", "ports": []any{},
			},
		},
		"paths": map[string]any{
			"magento_dir":   ".",
			"workdir":       "/code/shop",
			"strategy":      "bind mount",
			"compose_files": []any{"docker-compose.yml"},
		},
		"tooling": map[string]any{
			"machine":         "mac",
			"hm_version":      "2.0.0",
			"compose_version": "2.24.0",
			"xdebug":          "disabled",
		},
	}

	t.Run("Credentials nil", func(t *testing.T) {
		fake := answering(t)
		fake.description = describeFixture()

		stdout := &strings.Builder{}

		code := describe(nil, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("describe --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		if diff := cmp.Diff(base, envelope.Data); diff != "" {
			t.Fatalf("describe --json data (-want +got):\n%s", diff)
		}

		if _, ok := envelope.Data["credentials"]; ok {
			t.Fatalf("describe --json data has a credentials key, want it absent when the engine returned none")
		}
	})

	t.Run("--with-secrets", func(t *testing.T) {
		fake := answering(t)
		description := describeFixture()
		description.Credentials = credentialsFixture()
		fake.description = description

		stdout := &strings.Builder{}

		code := describe([]string{"--with-secrets"}, stdout, io.Discard, true)

		if code != exitOK {
			t.Fatalf("describe --with-secrets --json = %d, want exitOK", code)
		}

		var envelope struct {
			Data map[string]any `json:"data"`
		}

		if err := json.Unmarshal([]byte(stdout.String()), &envelope); err != nil {
			t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stdout.String(), err)
		}

		want := map[string]any{}
		for key, value := range base {
			want[key] = value
		}
		want["credentials"] = map[string]any{
			"database": map[string]any{
				"name":          "db-name-fixture",
				"user":          "db-user-fixture",
				"password":      "db-password-fixture",
				"root_password": "db-root-password-fixture",
			},
		}

		if diff := cmp.Diff(want, envelope.Data); diff != "" {
			t.Fatalf("describe --with-secrets --json data (-want +got):\n%s", diff)
		}
	})
}

// TestWhatDescribePrintsAsText covers describe's text rendering. Every want below is a
// hand-written literal: never a fmt.Sprintf of describe.go's own format string, which would
// compare the format with itself.
func TestWhatDescribePrintsAsText(t *testing.T) {
	t.Run("Credentials nil", func(t *testing.T) {
		// Also the nil-guard/panic test for describe.go:91: a removed guard dereferences a nil
		// *core.Credentials and panics this subtest instead of failing it quietly.
		fake := answering(t)
		fake.description = describeFixture()

		stdout := &strings.Builder{}

		code := describe(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("describe = %d, want exitOK", code)
		}

		want := "========================================\n" +
			"shop\n" +
			"========================================\n" +
			"\n" +
			"Environment\n" +
			"   status         running\n" +
			"   domain         shop.local\n" +
			"   worktree       feature-x\n" +
			"   root           /code/shop\n" +
			"\n" +
			"URLs\n" +
			"   base           https://shop.local/\n" +
			"   admin          https://shop.local/admin_1a2b3c\n" +
			"   mail           http://localhost:8025\n" +
			"   mailhog        http://localhost:8025\n" +
			"   search         http://localhost:9200\n" +
			"\n" +
			"Magento\n" +
			"   version        2.4.6\n" +
			"   mode           developer\n" +
			"   xdebug         disabled\n" +
			"\n" +
			"Services\n" +
			"   phpfpm         running    hiberusmagento/phpfpm:8.2\n" +
			"\n" +
			"Paths\n" +
			"   magento dir    .\n" +
			"   mounts         bind mount\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("describe text (-want +got):\n%s", diff)
		}
	})

	t.Run("Credentials set", func(t *testing.T) {
		fake := answering(t)
		description := describeFixture()
		description.Credentials = credentialsFixture()
		fake.description = description

		stdout := &strings.Builder{}

		code := describe([]string{"--with-secrets"}, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("describe --with-secrets = %d, want exitOK", code)
		}

		want := "========================================\n" +
			"shop\n" +
			"========================================\n" +
			"\n" +
			"Environment\n" +
			"   status         running\n" +
			"   domain         shop.local\n" +
			"   worktree       feature-x\n" +
			"   root           /code/shop\n" +
			"\n" +
			"URLs\n" +
			"   base           https://shop.local/\n" +
			"   admin          https://shop.local/admin_1a2b3c\n" +
			"   mail           http://localhost:8025\n" +
			"   mailhog        http://localhost:8025\n" +
			"   search         http://localhost:9200\n" +
			"\n" +
			"Magento\n" +
			"   version        2.4.6\n" +
			"   mode           developer\n" +
			"   xdebug         disabled\n" +
			"\n" +
			"Services\n" +
			"   phpfpm         running    hiberusmagento/phpfpm:8.2\n" +
			"\n" +
			"Paths\n" +
			"   magento dir    .\n" +
			"   mounts         bind mount\n" +
			"\n" +
			"Credentials\n" +
			"   db name        db-name-fixture\n" +
			"   db user        db-user-fixture\n" +
			"   db password    db-password-fixture\n" +
			"   db root_password db-root-password-fixture\n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("describe --with-secrets text (-want +got):\n%s", diff)
		}
	})

	t.Run("empty version, mode and URLs", func(t *testing.T) {
		// Also exercises orDash for an empty status, domain and worktree: a description this
		// sparse is what a project with nothing configured yet looks like.
		fake := answering(t)
		sparse := core.Description{}
		sparse.Project.Name = "gone"
		sparse.Paths.MagentoDir = "."
		fake.description = sparse

		stdout := &strings.Builder{}

		code := describe(nil, stdout, io.Discard, false)

		if code != exitOK {
			t.Fatalf("describe = %d, want exitOK", code)
		}

		want := "========================================\n" +
			"gone\n" +
			"========================================\n" +
			"\n" +
			"Environment\n" +
			"   status         -\n" +
			"   domain         -\n" +
			"   worktree       -\n" +
			"   root           \n" +
			"\n" +
			"URLs\n" +
			"\n" +
			"Magento\n" +
			"   version        unknown\n" +
			"   mode           unknown\n" +
			"   xdebug         \n" +
			"\n" +
			"Services\n" +
			"\n" +
			"Paths\n" +
			"   magento dir    .\n" +
			"   mounts         \n" +
			"\n"

		if diff := cmp.Diff(want, stdout.String()); diff != "" {
			t.Fatalf("describe text (-want +got):\n%s", diff)
		}
	})
}

// TestDescribeReportsAnUnresolvableProject is what a failed Describe becomes: exitProject and
// project_not_configured, with the engine's own reason interpolated rather than swallowed
// (describe.go:27-29), and a hint that names a real invocation — the unit-2 half of unit 0's
// knowing exception: describe.go:29's corrected hint shipped there and is only proved here,
// because reaching this branch needs the fake.
func TestDescribeReportsAnUnresolvableProject(t *testing.T) {
	cases := []struct {
		name       string
		binaryName string
		wantHint   string
	}{
		{"default binary", "hm", "hm setup"},
		{"a renamed binary", "hd", "hd setup"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.outcomes = []outcome{{err: errors.New("compose file is invalid")}}
			t.Setenv("COMMAND_BIN_NAME", tt.binaryName)

			stderr := &strings.Builder{}

			code := describe(nil, io.Discard, stderr, true)

			if code != exitProject {
				t.Fatalf("describe = %d, want exitProject", code)
			}

			var envelope struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
					Hint    string `json:"hint"`
				} `json:"error"`
			}

			if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON", stderr.String(), err)
			}

			if envelope.Error.Type != "project_not_configured" {
				t.Fatalf("describe error.type = %q, want %q", envelope.Error.Type, "project_not_configured")
			}

			if !strings.Contains(envelope.Error.Message, "compose file is invalid") {
				t.Fatalf("describe error.message = %q, want it to contain the engine's own error text", envelope.Error.Message)
			}

			if envelope.Error.Hint != tt.wantHint {
				t.Fatalf("describe error.hint = %q, want %q", envelope.Error.Hint, tt.wantHint)
			}
		})
	}
}

// describeFixture is a small builder for core.Description, the way app/describe.go:102-113
// itself builds one: Project, Magento, Data, Paths and Tooling are anonymous structs, so a
// fixture cannot be a single composite literal.
//
// RabbitMQ is the one URL left empty on purpose: it is what proves describeAsText skips a blank
// entry instead of printing a blank line, in the same fixture that otherwise exercises the
// ordinary rendering path.
func describeFixture() core.Description {
	description := core.Description{}
	description.Project.Name = "shop"
	description.Project.Root = "/code/shop"
	description.Project.Domain = "shop.local"
	description.Project.Worktree = "feature-x"
	description.Project.Status = "running"
	description.Project.URLs = core.URLs{
		Base:    "https://shop.local/",
		Admin:   "https://shop.local/admin_1a2b3c",
		Mail:    "http://localhost:8025",
		Mailhog: "http://localhost:8025",
		Search:  "http://localhost:9200",
	}

	description.Magento.Version = "2.4.6"
	description.Magento.Mode = "developer"

	description.Data.Anonymised = "yes"
	description.Data.AnonymisedAt = "2026-01-01T00:00:00Z"

	description.Services = []core.DescribedService{
		{Name: "phpfpm", Image: "hiberusmagento/phpfpm:8.2", State: "running", Ports: []string{}},
	}

	description.Paths.MagentoDir = "."
	description.Paths.Workdir = "/code/shop"
	description.Paths.Strategy = "bind mount"
	description.Paths.ComposeFiles = []string{"docker-compose.yml"}

	description.Tooling.Machine = "mac"
	description.Tooling.HmVersion = "2.0.0"
	description.Tooling.ComposeVersion = "2.24.0"
	description.Tooling.Xdebug = "disabled"

	return description
}

// credentialsFixture is invented by the test and read from nowhere, so nothing is leaked by
// construction. The four values are sentinels that cannot be mistaken for credentials —
// never the tool's own documented defaults (magento/magento, password), which would read like a
// real pair and would make a grep for them hit a test file.
func credentialsFixture() *core.Credentials {
	credentials := &core.Credentials{}
	credentials.Database.Name = "db-name-fixture"
	credentials.Database.User = "db-user-fixture"
	credentials.Database.Password = "db-password-fixture"
	credentials.Database.RootPassword = "db-root-password-fixture"

	return credentials
}
