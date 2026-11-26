package composelib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
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

// TestTheNetworksADownRemovesAreTheProjectsOwn: what Compose's down removes, by the name Docker
// gives them, sorted — the default network Compose adds for a service that names none, and the
// project's own declared ones. An external network, the proxy's, is somebody else's and stays.
func TestTheNetworksADownRemovesAreTheProjectsOwn(t *testing.T) {
	root := t.TempDir()
	compose := `services:
  db:
    image: mariadb:10.6
  nginx:
    image: nginx:1.28
    networks: [back, proxy]
networks:
  back: {}
  proxy:
    external: true
    name: hm-proxy
`
	if err := os.WriteFile(filepath.Join(root, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Orchestrator{}.Networks(core.Project{Name: "shop", Root: root},
		core.ComposeFiles{Load: []string{"docker-compose.yml"}})
	if err != nil {
		t.Fatalf("Networks = %v", err)
	}

	if want := []string{"shop_back", "shop_default"}; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("Networks = %q, want %q", got, want)
	}
}

// TestARecentLogIsWhatTheContainerWroteOnEitherStream: nginx logs its errors on stderr and its
// requests on stdout, and both are the service's log; what Compose says about the container — that
// it exited, that it is attaching — is not.
func TestARecentLogIsWhatTheContainerWroteOnEitherStream(t *testing.T) {
	log := &logLines{}

	log.Log("shop-nginx-1", "200 GET /")
	log.Err("shop-nginx-1", "[error] upstream timed out\n")
	log.Status("shop-nginx-1", "exited with code 1")

	if got, want := log.String(), "200 GET /\n[error] upstream timed out\n"; got != want {
		t.Fatalf("log = %q, want %q", got, want)
	}
}
