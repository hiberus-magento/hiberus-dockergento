package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What magento asks of the engine. magento (php.go:16) reaches the container only through
// inside (php.go:57), the same as purge and the other wrappers in inside_test.go, so Dir is the
// resolved project's root rather than the working directory, and the command is the Magento CLI
// with every argument handed on untouched. The shell twin it was measured against was
// console/commands/magento.sh: `exec.sh php ./bin/magento "$@"`, which execs into phpfpm as the
// default user (`git show f03a5e4:console/commands/magento.sh`).
//

// TestWhatMagentoAsks asserts Resolve first, then one Exec in phpfpm that runs php ./bin/magento
// followed by the arguments exactly as given — flags and values included, nothing reordered.
func TestWhatMagentoAsks(t *testing.T) {
	root := "/code/shop"

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"no arguments runs the bare CLI", nil,
			[]string{"php", "./bin/magento"}},
		{"a subcommand is passed on", []string{"cache:clean"},
			[]string{"php", "./bin/magento", "cache:clean"}},
		{"flags and values are passed on in order",
			[]string{"config:set", "--scope=default", "web/secure/base_url", "https://shop.test/"},
			[]string{"php", "./bin/magento", "config:set", "--scope=default", "web/secure/base_url",
				"https://shop.test/"}},
		{"-r is Magento's to read, not a root switch", []string{"-r"},
			[]string{"php", "./bin/magento", "-r"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}
			cwd := here()

			code := magento(tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("magento %v = %d, want exitOK", tt.args, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: "Exec", Dir: root, Service: phpService,
					Command: tt.want, Options: terminalOptions("")},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMagentoAnswersWithTheCLIsOwnStatus is inside's status branch (php.go:60-62): a failing
// Magento command's own exit code comes back as-is, never mapped into a Docker failure — a
// setup:upgrade that fails must fail the script that ran it with Magento's code. 42 is no code
// of this CLI's own contract (exitDocker is 3), so a status remapped to any of them shows here.
func TestMagentoAnswersWithTheCLIsOwnStatus(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {status: 42}}

	code := magento([]string{"setup:upgrade"}, io.Discard, io.Discard, false)

	if code != 42 {
		t.Fatalf("magento with a failed status = %d, want the CLI's own 42", code)
	}
}

// TestMagentoReportsAnEngineFailure is the other half of inside: a zero status with an error is
// the engine failing, not Magento, and it is reported as a Docker failure under --json.
func TestMagentoReportsAnEngineFailure(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	fake.outcomes = []outcome{{}, {err: errors.New("docker: no")}}

	stderr := &strings.Builder{}

	code := magento([]string{"cache:clean"}, io.Discard, stderr, true)

	if code != exitDocker {
		t.Fatalf("magento with a failed engine call = %d, want exitDocker", code)
	}

	var envelope struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("stderr is not a JSON envelope: %v\n%s", err, stderr.String())
	}

	if envelope.Command != "magento" {
		t.Errorf("envelope command = %q, want %q", envelope.Command, "magento")
	}
}

// TestMagentoOutsideAProjectAsksNothingOfTheContainer is projectOr's refusal (engine.go:91-95):
// a directory that resolves to no project answers exitProject, and Exec is never reached.
func TestMagentoOutsideAProjectAsksNothingOfTheContainer(t *testing.T) {
	fake := answering(t)
	cwd := here()

	code := magento([]string{"cache:clean"}, io.Discard, io.Discard, false)

	if code != exitProject {
		t.Fatalf("magento outside a project = %d, want exitProject", code)
	}

	want := []call{{Method: "Resolve", Dir: cwd}}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine outside a project (-want +got):\n%s", diff)
	}
}
