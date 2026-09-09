package app

import (
	"strings"
	"testing"
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
