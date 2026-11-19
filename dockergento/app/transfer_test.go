package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

type files struct {
	into []string

	// from is every copy out, as source and target: what the daemon was asked for, and where on
	// this machine it lands.
	from [][2]string
}

func (f *files) Into(container, source, target string) error {
	f.into = append(f.into, target)

	return nil
}

func (f *files) From(container, source, target string) error {
	f.from = append(f.from, [2]string{source, target})

	return nil
}

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

// Composer's macOS dance copies the whole workdir back out with From(".") — composer.sh's
// `docker cp phpfpm:$WORKDIR_PHP/. $MAGENTO_DIR` — and that has to read the workdir's contents
// into the Magento directory, not nest the workdir inside it.
func TestCopiarPuntoTraeElContenidoDelWorkdirAlDirectorioDeMagento(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	project.MagentoDir = "src"

	if err := os.Mkdir(filepath.Join(project.Root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := transfer.From(project, []string{"."}); err != nil {
		t.Fatal(err)
	}

	copied := transfer.Files.(*files).from
	want := [2]string{"/var/www/html/.", filepath.Join(project.Root, "src")}

	if len(copied) != 1 || copied[0] != want {
		t.Fatalf("copied %v, want only %v", copied, want)
	}
}

// transfer-db copies the dump it brought over SSH into the database container, which is the
// `docker cp db.sql.gz <db>:/tmp/db.sql.gz` of the shell: into the db service's container, not
// php's, and refused when that container is not running.
func TestUnFicheroSeCopiaAlContenedorDelServicioPedido(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: true},
		{ID: "db1", Project: "tienda", ComposeService: "db", Running: true},
	}}

	recorder := &recordingFiles{}
	transfer.Files = recorder

	if err := transfer.IntoService(project, "db", "/aqui/db.sql.gz", "/tmp"); err != nil {
		t.Fatal(err)
	}

	want := [3]string{"db1", "/aqui/db.sql.gz", "/tmp"}
	if len(recorder.into) != 1 || recorder.into[0] != want {
		t.Fatalf("copied %v, want only %v", recorder.into, want)
	}
}

func TestCopiarAUnServicioParadoSeNiega(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	recorder := &recordingFiles{}
	transfer.Files = recorder
	transfer.Binary = "hm"

	err := transfer.IntoService(project, "db", "/aqui/db.sql.gz", "/tmp")

	var refused core.Refusal
	if !errors.As(err, &refused) || refused.Kind != "service_not_running" || refused.Code != 5 ||
		refused.Message != "Service 'db' is not running" || refused.Hint != "hm start db" {
		t.Fatalf("err = %#v, want the service_not_running refusal for db", err)
	}

	if len(recorder.into) != 0 {
		t.Fatalf("copied %v into a container that is not running", recorder.into)
	}
}

// recordingFiles keeps the container each copy went to, which files does not.
type recordingFiles struct{ into [][3]string }

func (f *recordingFiles) Into(container, source, target string) error {
	f.into = append(f.into, [3]string{container, source, target})

	return nil
}

func (f *recordingFiles) From(string, string, string) error { return nil }
