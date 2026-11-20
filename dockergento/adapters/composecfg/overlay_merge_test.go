package composecfg

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The worktree overlay the binary writes, merged by Compose's own library over a project that
// bind mounts its code: the main checkout's dependencies land read-only where PHP looks for them,
// on PHP only, and the code mount survives underneath. This is what the shell suite checked
// against the shell writer; it is Compose's merge that it depends on, so it is Compose's library
// that answers.
func TestTheWorktreeOverlayMountsTheDependenciesOverTheCode(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(t.TempDir(), "main")

	base := `services:
  phpfpm:
    image: hiberusmagento/php:8.3-bookworm
    volumes:
      - ./:/var/www/html
  nginx:
    image: hiberusmagento/nginx:1.28
    volumes:
      - ./:/var/www/html
`
	overlay := core.OverlayFor("hm", "tienda-feature-x", "agent", "feature-x.shop.test", "hm-gateway",
		[]string{"phpfpm", "nginx"}, []string{main + "/vendor:/var/www/html/vendor:ro"})

	for name, content := range map[string]string{"docker-compose.yml": base, "overlay.yml": overlay} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	options, err := cli.NewProjectOptions(
		[]string{filepath.Join(root, "docker-compose.yml"), filepath.Join(root, "overlay.yml")},
		cli.WithWorkingDirectory(root), cli.WithName("tienda-feature-x"), cli.WithResolvedPaths(true))
	if err != nil {
		t.Fatal(err)
	}

	project, err := options.LoadProject(context.Background())
	if err != nil {
		t.Fatalf("LoadProject = %v\noverlay:\n%s", err, overlay)
	}

	byTarget := func(service, target string) []types.ServiceVolumeConfig {
		var found []types.ServiceVolumeConfig
		for _, volume := range project.Services[service].Volumes {
			if volume.Target == target {
				found = append(found, volume)
			}
		}

		return found
	}

	vendor := byTarget("phpfpm", "/var/www/html/vendor")
	if len(vendor) != 1 || vendor[0].Source != main+"/vendor" || !vendor[0].ReadOnly {
		t.Errorf("phpfpm vendor mounts = %+v, want the main checkout's, read-only", vendor)
	}

	if code := byTarget("phpfpm", "/var/www/html"); len(code) != 1 {
		t.Errorf("phpfpm code mounts = %+v, want the code mount still there underneath", code)
	}

	if other := byTarget("nginx", "/var/www/html/vendor"); len(other) != 0 {
		t.Errorf("nginx vendor mounts = %+v, want none", other)
	}
}
