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
// -t/--timeout on stop and restart: how long Compose gives each container to stop before it kills
// it. stop.sh handed its options to `docker compose stop`, and the port took them for services;
// both commands now read the option in the four spellings Compose's own flag parser accepts, and
// restart, which is a stop and then a start, gives it to its stop.
//

func seconds(n int) *int {
	return &n
}

func TestStopAndRestartPassTheTimeout(t *testing.T) {
	forms := []struct {
		name string
		args []string
	}{
		{"-t N", []string{"-t", "5"}},
		{"-t=N", []string{"-t=5"}},
		{"--timeout N", []string{"--timeout", "5"}},
		{"--timeout=N", []string{"--timeout=5"}},
	}

	for _, command := range []string{"stop", "restart"} {
		for _, form := range forms {
			t.Run(command+" "+form.name, func(t *testing.T) {
				fake := answering(t)
				fake.project = core.Project{Name: "shop", Root: "/code/shop"}
				cwd := here()

				args := append([]string{command}, form.args...)
				if code := Run(args, io.Discard, io.Discard); code != exitOK {
					t.Fatalf("%v = %d, want exitOK", args, code)
				}

				want := []call{
					{Method: "Resolve", Dir: cwd},
					{Method: strings.ToUpper(command[:1]) + command[1:], Dir: cwd,
						Services: []string{}, Timeout: seconds(5)},
				}

				if diff := cmp.Diff(want, fake.calls); diff != "" {
					t.Fatalf("asked of the engine (-want +got):\n%s", diff)
				}
			})
		}
	}
}

// Zero is a timeout, not its absence: Compose kills at once, where no option leaves the wait to
// Compose's own default.
func TestAZeroTimeoutIsPassedOn(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: "/code/shop"}
	cwd := here()

	if code := Run([]string{"stop", "-t", "0"}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("stop -t 0 = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Stop", Dir: cwd, Services: []string{}, Timeout: seconds(0)},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

func TestTheTimeoutLeavesTheServicesAndTheSnapshotAlone(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want call
	}{
		{"stop, services around the option", []string{"stop", "nginx", "-t", "5", "phpfpm"},
			call{Method: "Stop", Services: []string{"nginx", "phpfpm"}, Timeout: seconds(5)}},
		{"stop, with --snapshot", []string{"stop", "--snapshot", "--timeout=30", "db"},
			call{Method: "Stop", Services: []string{"db"}, Snapshot: true, Timeout: seconds(30)}},
		{"restart, services around the option", []string{"restart", "nginx", "--timeout", "5", "phpfpm"},
			call{Method: "Restart", Services: []string{"nginx", "phpfpm"}, Timeout: seconds(5)}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			if code := Run(tt.args, io.Discard, io.Discard); code != exitOK {
				t.Fatalf("%v = %d, want exitOK", tt.args, code)
			}

			tt.want.Dir = cwd
			want := []call{{Method: "Resolve", Dir: cwd}, tt.want}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// Without the option the stop is exactly the one before it: no timeout reaches the engine, and
// Compose waits as long as it always did.
func TestWithoutTheOptionThereIsNoTimeout(t *testing.T) {
	for _, command := range []string{"stop", "restart"} {
		t.Run(command, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			cwd := here()

			if code := Run([]string{command, "phpfpm"}, io.Discard, io.Discard); code != exitOK {
				t.Fatalf("%s phpfpm = %d, want exitOK", command, code)
			}

			want := []call{
				{Method: "Resolve", Dir: cwd},
				{Method: strings.ToUpper(command[:1]) + command[1:], Dir: cwd, Services: []string{"phpfpm"}},
			}

			if diff := cmp.Diff(want, fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}

// usageFault is the error of a JSON envelope, decoded.
type usageFault struct {
	Code                int
	Type, Message, Hint string
}

// fault runs args under --json and answers the error of the envelope it printed, decoded: the
// envelope escapes < and >, so its bytes are not what a hint reads.
func fault(t *testing.T, args []string) (int, usageFault) {
	t.Helper()

	stderr := &strings.Builder{}
	code := Run(append([]string{"--json"}, args...), io.Discard, stderr)

	var envelope struct {
		Error struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(stderr.String()), &envelope); err != nil {
		t.Fatalf("%v: stderr is not a JSON envelope: %v\n%s", args, err, stderr.String())
	}

	return code, usageFault(envelope.Error)
}

func TestATimeoutThatIsNotOneIsAUsageError(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"stop, -t with nothing after it", []string{"stop", "-t"}, "-t needs a number of seconds"},
		{"stop, --timeout= with nothing", []string{"stop", "--timeout="}, "--timeout needs a number of seconds"},
		{"restart, --timeout with nothing after it", []string{"restart", "--timeout"}, "--timeout needs a number of seconds"},
		{"restart, -t= with nothing", []string{"restart", "-t="}, "-t needs a number of seconds"},
		{"stop, a word", []string{"stop", "-t", "five"}, "The timeout is a number of seconds: five"},
		{"stop, a negative number", []string{"stop", "--timeout=-3"}, "The timeout is a number of seconds: -3"},
		{"restart, a fraction", []string{"restart", "-t", "1.5"}, "The timeout is a number of seconds: 1.5"},
		{"restart, a service where the number goes", []string{"restart", "-t", "phpfpm"}, "The timeout is a number of seconds: phpfpm"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			t.Setenv("COMMAND_BIN_NAME", "hm")

			code, got := fault(t, tt.args)

			want := usageFault{
				exitUsage, "invalid_argument", tt.message,
				"hm " + tt.args[0] + " [-t seconds] [service...]",
			}

			if code != exitUsage || got != want {
				t.Fatalf("%v = %d %+v, want %d %+v", tt.args, code, got, exitUsage, want)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("%v asked the engine %v, want nothing", tt.args, fake.calls)
			}
		})
	}
}

// Any other option is refused before anything is resolved, as restart always refused one, rather
// than handed to Compose as a service name.
func TestAnyOtherOptionIsRefused(t *testing.T) {
	for _, command := range []string{"stop", "restart"} {
		t.Run(command, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: "/code/shop"}
			t.Setenv("COMMAND_BIN_NAME", "hm")

			code, got := fault(t, []string{command, "phpfpm", "-x"})

			want := usageFault{
				exitUsage, "invalid_argument", "Unknown option: -x",
				"hm " + command + " [-t seconds] [service...]",
			}

			if code != exitUsage || got != want {
				t.Fatalf("%s -x = %d %+v, want %d %+v", command, code, got, exitUsage, want)
			}

			if len(fake.calls) != 0 {
				t.Fatalf("%s -x asked the engine %v, want nothing", command, fake.calls)
			}
		})
	}
}
