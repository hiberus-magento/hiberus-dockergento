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

