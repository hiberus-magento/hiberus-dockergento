package composelib

import (
	"testing"
	"time"

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

// TestAStopTimeoutReachesCompose: the seconds asked for are Compose's StopOptions.Timeout, and no
// timeout leaves it nil, which is Compose's own default — what every stop waited before the option.
func TestAStopTimeoutReachesCompose(t *testing.T) {
	loaded := &types.Project{Name: "shop"}
	five, zero := 5, 0

	cases := []struct {
		name    string
		timeout *int
		want    *time.Duration
	}{
		{"no timeout is Compose's default", nil, nil},
		{"seconds become a duration", &five, durationOf(5 * time.Second)},
		{"zero is a timeout, not its absence", &zero, durationOf(0)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := stopOptions(loaded, []string{"phpfpm"}, tt.timeout)

			if got.Project != loaded || len(got.Services) != 1 || got.Services[0] != "phpfpm" {
				t.Fatalf("stop options = %+v, want the project and the services passed on", got)
			}

			switch {
			case tt.want == nil && got.Timeout != nil:
				t.Fatalf("timeout = %v, want nil", *got.Timeout)
			case tt.want != nil && (got.Timeout == nil || *got.Timeout != *tt.want):
				t.Fatalf("timeout = %v, want %v", got.Timeout, *tt.want)
			}
		})
	}
}

func durationOf(d time.Duration) *time.Duration {
	return &d
}
