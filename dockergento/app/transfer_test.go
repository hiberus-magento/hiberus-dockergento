package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

type files struct{ into []string }

func (f *files) Into(container, source, target string) error {
	f.into = append(f.into, target)

	return nil
}

func (*files) From(container, source, target string) error { return nil }

// transferencia sets up a project with a vendor directory and a running php container, and keeps
// apart the runner that works as the container's own user and the one that works as root.
func transferencia(t *testing.T, comun, root *runner) (Transfer, core.Project) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}

	containers := []core.Container{{ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: true}}

	return Transfer{
		Engine: &engine{containers: containers},
		Files:  &files{},
		Runner: comun,
		Root:   root,
	}, core.Project{Name: "tienda", Root: dir}
}

func TestLaPropiedadSeArreglaComoRoot(t *testing.T) {
	comun, root := &runner{}, &runner{}
	transfer, project := transferencia(t, comun, root)

	if err := transfer.Into(project, []string{"vendor"}, "app:app"); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(root.command, " "); !strings.Contains(got, "chown -R app:app /var/www/html/vendor") {
		t.Fatalf("root ran %q, want the chown", got)
	}

	if got := strings.Join(comun.command, " "); strings.Contains(got, "chown") {
		t.Fatalf("the container's own user ran %q, and it cannot chown what root owns", got)
	}
}

func TestElCodigoDeSalidaDelChownNoSeIgnora(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{status: 1})

	err := transfer.Into(project, []string{"vendor"}, "app:app")
	if err == nil {
		t.Fatal("a chown that failed was reported as success")
	}

	if !strings.Contains(err.Error(), "chown") || !strings.Contains(err.Error(), "/var/www/html/vendor") {
		t.Fatalf("error %q should name the chown and its target", err)
	}
}

func TestElCodigoDeSalidaDelChownNoSeIgnoraAlCopiarTodo(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{status: 1})

	if err := transfer.All(project, "app:app"); err == nil {
		t.Fatal("a chown that failed was reported as success")
	}
}

func TestElCodigoDeSalidaDelMkdirNoSeIgnora(t *testing.T) {
	transfer, project := transferencia(t, &runner{status: 1}, &runner{})

	err := transfer.Into(project, []string{"vendor"}, "app:app")
	if err == nil || !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("error = %v, want the failed mkdir named", err)
	}
}
