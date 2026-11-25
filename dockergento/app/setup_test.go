package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/fsprops"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// setup.go:516-530 is not edited by this pin — the step list already exists, and its order is a
// fact about the shell implementation it replaces, not something this commit changes. Neither
// `go test ./...` nor `tests/run.sh tests/unit` runs `go_setup_test.sh`, so a silent reorder there
// would otherwise land green.
func TestTheInstallStepsRunInOneOrderThroughOneDoor(t *testing.T) {
	joined := func(t *testing.T, steps [][]string) []string {
		t.Helper()

		out := make([]string, len(steps))
		for i, step := range steps {
			out[i] = strings.Join(step, " ")
		}

		return out
	}

	assertOrder := func(t *testing.T, got []string, want []string) {
		t.Helper()

		if len(got) != len(want) {
			t.Fatalf("steps run = %v, want %v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("steps run = %v, want %v", got, want)
			}
		}
	}

	t.Run("with no dump", func(t *testing.T) {
		legacy := &shell{}
		setup := Setup{Legacy: legacy, Binary: "hm"}

		if err := setup.install(Decided{Domain: "tienda.test"}); err != nil {
			t.Fatalf("= %v, want no error", err)
		}

		assertOrder(t, joined(t, legacy.ran), []string{
			"start",
			"composer install",
			"install",
			"magento setup:upgrade",
			"magento deploy:mode:set developer",
			"restart",
			"ssl tienda.test",
			"set-host tienda.test --no-database",
		})
	})

	t.Run("with a dump", func(t *testing.T) {
		legacy := &shell{}
		setup := Setup{Legacy: legacy, Binary: "hm"}

		if err := setup.install(Decided{Domain: "tienda.test", Dump: "/tmp/volcado.sql"}); err != nil {
			t.Fatalf("= %v, want no error", err)
		}

		assertOrder(t, joined(t, legacy.ran), []string{
			"start",
			"composer install",
			"mysql -i /tmp/volcado.sql",
			"mysql -q DELETE FROM admin_user;",
			"install",
			"magento setup:upgrade",
			"magento deploy:mode:set developer",
			"restart",
			"ssl tienda.test",
			"set-host tienda.test --no-database",
		})
	})
}

// --use-default reaches Magento's own installation too: without it, `install` asked the questions
// setup had just been told not to ask.
func TestTheInstallStepTakesItsDefaultsWhenSetupWasToldTo(t *testing.T) {
	tests := []struct {
		name       string
		useDefault bool
		want       string
	}{
		{"with --use-default", true, "install -u"},
		{"without it", false, "install"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacy := &shell{}
			setup := Setup{Properties: fsprops.Reader{}, Legacy: legacy, Binary: "hm"}

			decided, err := setup.Decide(projectDir(t, ""), core.SetupOptions{
				ProjectName: "tienda", Domain: "tienda.test", Root: ".", Install: true, UseDefault: tt.useDefault,
			}, false)
			if err != nil {
				t.Fatalf("Decide = %v, want no error", err)
			}

			if err := setup.install(decided); err != nil {
				t.Fatalf("install = %v, want no error", err)
			}

			if got := strings.Join(legacy.ran[2], " "); got != tt.want {
				t.Fatalf("third step = %q, want %q (steps: %v)", got, tt.want, legacy.ran)
			}
		})
	}
}

//
// What setup records about the project's name.
//
// properties.json is committed, so whatever it says travels to every clone. Writing down the name
// the directory would have given anyway is what made a second clone inherit the first one's
// containers and volumes. These run against the real properties adapter because the rule is about
// what ends up in the file.
//

// projectDir is a checkout whose directory name is the name setup would propose.
func projectDir(t *testing.T, properties string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "tienda")

	if err := os.MkdirAll(filepath.Join(dir, "config", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}

	if properties != "" {
		file := filepath.Join(dir, "config", "docker", "properties.json")
		if err := os.WriteFile(file, []byte(properties), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func savedProperties(t *testing.T, dir string, decided Decided) map[string]string {
	t.Helper()

	if err := (Setup{Properties: fsprops.Reader{}}).Save(dir, decided); err != nil {
		t.Fatalf("Save = %v, want no error", err)
	}

	saved, err := fsprops.Reader{}.Load(dir)
	if err != nil {
		t.Fatalf("reading back = %v, want no error", err)
	}

	return saved
}

func TestAcceptingTheProposedNameRecordsNoName(t *testing.T) {
	dir := projectDir(t, "")

	saved := savedProperties(t, dir, Decided{
		Name: core.DeriveProjectName(dir), Domain: "tienda.test", MagentoDir: ".", Mail: "mailhog",
	})

	if name, ok := saved["COMPOSE_PROJECT_NAME"]; ok {
		t.Fatalf("COMPOSE_PROJECT_NAME = %q, want nothing recorded for the derived name", name)
	}

	// The rest of what was decided is still written, so the empty name is not an empty save
	if saved["DOMAIN"] != "tienda.test" {
		t.Fatalf("DOMAIN = %q, want tienda.test", saved["DOMAIN"])
	}
}

func TestANameOfYourOwnIsRecorded(t *testing.T) {
	dir := projectDir(t, "")

	saved := savedProperties(t, dir, Decided{
		Name: "mi-nombre", Domain: "tienda.test", MagentoDir: ".", Mail: "mailhog",
	})

	if saved["COMPOSE_PROJECT_NAME"] != "mi-nombre" {
		t.Fatalf("COMPOSE_PROJECT_NAME = %q, want the chosen name recorded", saved["COMPOSE_PROJECT_NAME"])
	}
}

func TestAProjectThatAlreadyHadANameKeepsIt(t *testing.T) {
	// Even when the name it had is the one the directory would give: it was written down once,
	// and a later setup pressing enter through the questions must not take it away
	dir := projectDir(t, `{"COMPOSE_PROJECT_NAME": "tienda"}`)

	saved := savedProperties(t, dir, Decided{
		Name: "tienda", Domain: "tienda.test", MagentoDir: ".", Mail: "mailhog",
	})

	if saved["COMPOSE_PROJECT_NAME"] != "tienda" {
		t.Fatalf("COMPOSE_PROJECT_NAME = %q, want the existing name kept", saved["COMPOSE_PROJECT_NAME"])
	}
}

func TestSavingDoesNotWipeTheOtherProperties(t *testing.T) {
	dir := projectDir(t, `{"BIN_DIR": "custom/bin"}`)

	saved := savedProperties(t, dir, Decided{
		Name: core.DeriveProjectName(dir), Domain: "tienda.test", MagentoDir: ".", Mail: "mailhog",
	})

	if saved["BIN_DIR"] != "custom/bin" {
		t.Fatalf("BIN_DIR = %q, want custom/bin left alone", saved["BIN_DIR"])
	}
}
