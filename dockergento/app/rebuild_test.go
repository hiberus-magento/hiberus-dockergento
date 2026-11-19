package app

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// rebuild: a stop, `up --build`, the bind-mount check when the whole environment was rebuilt, and
// then what the platform needs after starting — rebuild.sh's sequence, with the stop and the steps
// after it no longer going through the shell.
//

// boundVendor is a running php container whose vendor directory comes from the host, which the
// bind-mount check refuses on macOS: it is how a test sees whether the check ran at all.
func boundVendor() []core.Container {
	return []core.Container{{
		ID: "phpfpm-1", Running: true, ComposeProject: "tienda", ComposeService: "phpfpm",
		Mounts: []core.Mount{{Type: "bind", Destination: "/var/www/html/vendor"}},
	}}
}

func TestRebuildingTheWholeEnvironmentBuildsNginxAndChecksTheMounts(t *testing.T) {
	log := []string{}
	operator := operatorWith(boundVendor(), &orchestrator{log: &log}, &shell{})

	refusal := refusalOf(t, operator.Rebuild(linuxProject, core.ComposeFiles{}, nil))

	if refusal.Kind != "vendor_is_bound" {
		t.Fatalf("refusal = %+v, want the bind-mount check to have run after the build", refusal)
	}

	if diff := cmp.Diff([]string{"stop ", "up --build nginx"}, log); diff != "" {
		t.Fatalf("done (-want +got):\n%s", diff)
	}
}

func TestRebuildingOnLinuxRunsTheStepsAfterStarting(t *testing.T) {
	log := []string{}
	legacy := &shell{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, legacy, "1000", "1000", nil)

	if err := operator.Rebuild(linuxProject, core.ComposeFiles{}, nil); err != nil {
		t.Fatalf("Rebuild = %v, want no error", err)
	}

	want := append(append([]string{"stop ", "up --build nginx", "wait 5s"}, idsRead()...), selfRouting()...)

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("done (-want +got):\n%s", diff)
	}

	if len(legacy.ran) != 0 {
		t.Fatalf("asked of the shell = %v, want nothing", legacy.ran)
	}
}

// TestRebuildingServicesStopsBuildsThoseAndStillChecksTheMounts: rebuild.sh ran the bind-mount
// check after every build, services named or not, and so does this.
func TestRebuildingServicesStopsBuildsThoseAndStillChecksTheMounts(t *testing.T) {
	log := []string{}
	operator := operatorWith(boundVendor(), &orchestrator{log: &log}, &shell{})

	refusal := refusalOf(t, operator.Rebuild(linuxProject, core.ComposeFiles{}, []string{"phpfpm", "nginx"}))
	if refusal.Kind != "vendor_is_bound" {
		t.Fatalf("refusal = %+v, want the bind-mount check to have run after building the services", refusal)
	}

	if diff := cmp.Diff([]string{"stop phpfpm nginx", "up --build phpfpm nginx"}, log); diff != "" {
		t.Fatalf("done (-want +got):\n%s", diff)
	}
}

func TestRebuildIsRefusedFromAWorktreeWithNoEnvironmentOfItsOwn(t *testing.T) {
	log := []string{}
	operator := operatorWith(nil, &orchestrator{log: &log}, &shell{})
	project := core.Project{Name: "tienda", Root: "/code/tienda", InWorktree: true}

	refusal := refusalOf(t, operator.Rebuild(project, core.ComposeFiles{}, nil))

	if refusal.Kind != "blocked_in_worktree" || refusal.Code != 6 {
		t.Fatalf("refusal = %+v, want blocked_in_worktree with code 6", refusal)
	}

	if len(log) != 0 {
		t.Fatalf("done before refusing = %v, want nothing", log)
	}
}

func TestAFailingStopEndsTheRebuildThere(t *testing.T) {
	log := []string{}
	failure := errors.New("the daemon went away")
	operator := operatorWith(nil, &orchestrator{log: &log, stopErr: failure}, &shell{})

	if err := operator.Rebuild(linuxProject, core.ComposeFiles{}, []string{"phpfpm"}); !errors.Is(err, failure) {
		t.Fatalf("Rebuild with a failing stop = %v, want the stop's failure", err)
	}

	if diff := cmp.Diff([]string{"stop phpfpm"}, log); diff != "" {
		t.Fatalf("done (-want +got):\n%s", diff)
	}
}
