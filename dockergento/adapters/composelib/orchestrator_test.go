package composelib

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

// TestBuildingFirstAppliesOnlyToServicesWithABuild: `up --build` is Compose setting the build pull
// policy on every service that has something to build, and only on those — an image somebody
// pulls has no build section, and asking to build it would fail.
func TestBuildingFirstAppliesOnlyToServicesWithABuild(t *testing.T) {
	loaded := &types.Project{Services: types.Services{
		"phpfpm": {Name: "phpfpm", Build: &types.BuildConfig{Context: "."}},
		"nginx":  {Name: "nginx", Image: "nginx:1.25"},
	}}

	buildFirst(loaded)

	if got := loaded.Services["phpfpm"].PullPolicy; got != types.PullPolicyBuild {
		t.Errorf("phpfpm pull policy = %q, want %q", got, types.PullPolicyBuild)
	}

	if got := loaded.Services["nginx"].PullPolicy; got != "" {
		t.Errorf("nginx pull policy = %q, want it left alone", got)
	}
}
