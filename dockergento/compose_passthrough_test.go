package dockergento

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The passthrough creates containers as much as start does — `hm docker-compose up -d` is the
// obvious case — so it has to hand Compose the same labels the library stamps. Without the two
// that cost something to resolve, the containers it creates carry an empty version, read to the
// inventory as made before the labels existed, and hash differently from the library's.
func TestComposePassthroughHandsComposeTheCreationLabels(t *testing.T) {
	bin := t.TempDir()
	dump := filepath.Join(t.TempDir(), "environment")

	// A docker that is Compose: it answers the version probe and writes down what it was given
	script := "#!/bin/sh\nif [ \"$2\" = version ]; then exit 0; fi\nenv > \"$HM_TEST_DUMP\"\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing the fake docker = %v, want no error", err)
	}

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HM_TEST_DUMP", dump)
	t.Setenv("HM_MAGENTO", "")

	project := proyecto(t, `{"COMPOSE_PROJECT_NAME": "tienda", "MAGENTO_DIR": "."}`, "docker-compose.yml")

	lock := `{"packages": [{"name": "magento/product-community-edition", "version": "2.4.7"}]}`
	if err := os.WriteFile(filepath.Join(project.Root, "composer.lock"), []byte(lock), 0o644); err != nil {
		t.Fatalf("writing the composer.lock = %v, want no error", err)
	}

	if status, err := motor(t).Compose(project.Root, []string{"up", "-d"}); err != nil || status != 0 {
		t.Fatalf("Compose() = %d, %v, want 0, no error", status, err)
	}

	contents, err := os.ReadFile(dump)
	if err != nil {
		t.Fatalf("reading what Compose was given = %v, want no error", err)
	}

	// Only the tool's own variables: the rest is whoever runs the test's environment
	labels := map[string]string{}
	for _, line := range strings.Split(string(contents), "\n") {
		if key, value, found := strings.Cut(line, "="); found && strings.HasPrefix(key, "HM_") {
			labels[key] = value
		}
	}

	if labels["HM_PROJECT"] != "tienda" {
		t.Errorf("HM_PROJECT = %q, want %q", labels["HM_PROJECT"], "tienda")
	}

	if labels["HM_MAGENTO"] != "2.4.7" {
		t.Errorf("HM_MAGENTO = %q, want %q", labels["HM_MAGENTO"], "2.4.7")
	}

	if _, found := labels["HM_VERSION"]; !found {
		t.Errorf("HM_VERSION was not given at all; the labels were %v", labels)
	}
}
