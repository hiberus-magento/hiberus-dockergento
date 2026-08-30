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
