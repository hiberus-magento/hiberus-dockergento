package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

// The passthrough is the whole of this stage, so what it has to get right is small and worth
// checking: find the shell tree, run it, and give back what it gave.

func shellTree(t *testing.T, script string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "console"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "run"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestArgumentsArriveUntouched(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$0.args\"\n")

	if _, err := (Runner{Root: root}).Run([]string{"magento", "cache:flush", "--json"}); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	arguments, err := os.ReadFile(filepath.Join(root, "bin", "run.args"))
	if err != nil {
		t.Fatal(err)
	}

	if string(arguments) != "magento\ncache:flush\n--json\n" {
		t.Fatalf("los argumentos llegaron distintos: %q", arguments)
	}
}

// The exit code is the contract: 2 is a usage error, 6 is a refusal on purpose, and a wrapper
// that flattened them into 1 would break every caller that branches on them.
func TestTheExitCodeIsTheOneTheShellGave(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nexit 6\n")

	code, err := (Runner{Root: root}).Run([]string{"down"})
	if err != nil {
		t.Fatalf("un código de salida no es un error del proceso: %v", err)
	}

	if code != 6 {
		t.Fatalf("código inesperado: %d", code)
	}
}

func TestSuccessIsZero(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nexit 0\n")

	code, _ := (Runner{Root: root}).Run([]string{"list"})
	if code != 0 {
		t.Fatalf("código inesperado: %d", code)
	}
}

func TestAMissingShellTreeIsReportedRatherThanGuessed(t *testing.T) {
	root := t.TempDir()

	if _, err := (Runner{Root: filepath.Join(root, "no-existe")}).Run([]string{"list"}); err == nil {
		t.Fatal("debería decir que no lo encuentra")
	}
}

func TestTheOverrideWinsOverEverythingElse(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nexit 0\n")
	t.Setenv("HM_LEGACY_ROOT", filepath.Join(root, "no-existe"))

	located, err := (Runner{Root: root}).locate()
	if err != nil || located != root {
		t.Fatalf("el campo debería ganar al entorno: %q, %v", located, err)
	}
}

func TestTheEnvironmentPointsItSomewhereElse(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nexit 0\n")
	t.Setenv("HM_LEGACY_ROOT", root)

	located, err := (Runner{}).locate()
	if err != nil || located != root {
		t.Fatalf("debería usar el entorno: %q, %v", located, err)
	}
}

// The registration is what makes a bridged command run from a branch environment stay in it: the
// shell half cannot read the registry, so it is handed what the registry said.
func TestTheRegistrationArrives(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nprintf '%s\\n' \"$HM_REGISTERED\" > \"$0.env\"\n")

	handed := []string{"HM_REGISTERED=tienda/rama", "HM_REGISTERED_PROJECT=tienda-rama"}

	if _, err := (Runner{Root: root, Registration: handed}).Run([]string{"setup"}); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	given, err := os.ReadFile(filepath.Join(root, "bin", "run.env"))
	if err != nil {
		t.Fatal(err)
	}

	if string(given) != "tienda/rama\n" {
		t.Fatalf("la registración llegó distinta: %q", given)
	}
}

// And one inherited from an outer invocation does not survive beside it. Two entries for the same
// name in one environment is a question of whose reading wins, and this is not a question worth
// having: the answer would be a branch environment's identity used in the main checkout.
func TestAnInheritedRegistrationIsNotCarriedOver(t *testing.T) {
	root := shellTree(t, "#!/bin/sh\nprintf '%s\\n' \"$HM_REGISTERED\" > \"$0.env\"\n")

	t.Setenv("HM_REGISTERED", "otra/vieja")
	t.Setenv("HM_REGISTERED_PROJECT", "otra-vieja")

	if _, err := (Runner{Root: root}).Run([]string{"setup"}); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	given, err := os.ReadFile(filepath.Join(root, "bin", "run.env"))
	if err != nil {
		t.Fatal(err)
	}

	if string(given) != "\n" {
		t.Fatalf("no debería quedar nada de la de fuera: %q", given)
	}
}
