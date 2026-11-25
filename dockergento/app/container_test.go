package app

import (
	"errors"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The one resolution of a service to its container, shared by every caller: the commands that run
// something inside it, and the copies in and out.
//

// An orphaned stopped container listed before the live one must not hide it: resolution keeps
// looking for a running one rather than giving up at the first stopped match.
func TestARunningContainerIsFoundAfterAStoppedOne(t *testing.T) {
	containers := &engine{containers: []core.Container{
		{ID: "old", Project: "tienda", ComposeService: "db", Running: false},
		{ID: "live", Project: "tienda", ComposeService: "db", Running: true},
	}}

	id, err := resolveContainer(containers, core.Project{Name: "tienda"}, "db", "hm")
	if err != nil || id != "live" {
		t.Fatalf("resolveContainer = %q, %v; want the running container", id, err)
	}
}

// Only stopped ones is still a refusal, with the service named, as it always was.
func TestOnlyStoppedContainersAreStillRefused(t *testing.T) {
	containers := &engine{containers: []core.Container{
		{ID: "old", Project: "tienda", ComposeService: "db", Running: false},
	}}

	_, err := resolveContainer(containers, core.Project{Name: "tienda"}, "db", "hm")

	var refused core.Refusal
	if !errors.As(err, &refused) || refused.Kind != "service_not_running" || refused.Code != 5 ||
		refused.Message != "Service 'db' is not running" || refused.Hint != "hm start db" {
		t.Fatalf("resolveContainer = %#v, want the service_not_running refusal", err)
	}
}

// Where stopped is enough — reading a container's filesystem — a running one is still preferred,
// a stopped one is taken when it is all there is, and none at all says so.
func TestAContainerThatMayBeStopped(t *testing.T) {
	tests := []struct {
		name        string
		containers  []core.Container
		wantID      string
		wantRunning bool
		wantMessage string
	}{
		{
			name: "a running one wins over a stopped one listed first",
			containers: []core.Container{
				{ID: "old", Project: "tienda", ComposeService: "phpfpm"},
				{ID: "live", Project: "tienda", ComposeService: "phpfpm", Running: true},
			},
			wantID: "live", wantRunning: true,
		},
		{
			name:       "a stopped one when it is all there is",
			containers: []core.Container{{ID: "old", Project: "tienda", ComposeService: "phpfpm"}},
			wantID:     "old",
		},
		{
			name:        "none at all",
			containers:  []core.Container{{ID: "db1", Project: "tienda", ComposeService: "db", Running: true}},
			wantMessage: "This project has no phpfpm container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := anyContainer(&engine{containers: tt.containers}, core.Project{Name: "tienda"}, "phpfpm", "hm")

			if tt.wantMessage != "" {
				var refused core.Refusal
				if !errors.As(err, &refused) || refused.Kind != "service_not_running" || refused.Code != 5 ||
					refused.Message != tt.wantMessage || refused.Hint != "hm start phpfpm" {
					t.Fatalf("anyContainer = %#v, want the refusal saying there is none", err)
				}

				return
			}

			if err != nil || found.ID != tt.wantID || found.Running != tt.wantRunning {
				t.Fatalf("anyContainer = %+v, %v; want %s (running %v)", found, err, tt.wantID, tt.wantRunning)
			}
		})
	}
}

// The engine's own error is passed on, not turned into a refusal.
func TestAnEngineErrorIsPassedOn(t *testing.T) {
	broken := errors.New("cannot connect to the docker daemon")

	if _, err := resolveContainer(&engine{err: broken}, core.Project{Name: "tienda"}, "db", "hm"); !errors.Is(err, broken) {
		t.Fatalf("resolveContainer = %v, want the engine's error", err)
	}

	if _, err := anyContainer(&engine{err: broken}, core.Project{Name: "tienda"}, "db", "hm"); !errors.Is(err, broken) {
		t.Fatalf("anyContainer = %v, want the engine's error", err)
	}
}
