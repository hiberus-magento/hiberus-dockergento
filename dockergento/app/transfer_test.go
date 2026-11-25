package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
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

// Composer's macOS dance stops phpfpm before copying the workdir back out, so that nothing writes
// to the tree while it is read: copying out of a container is a read of its filesystem, which the
// daemon serves whether it runs or not, and refusing it left the host's vendor cleared and empty.
func TestCopyingOutOfAStoppedContainerWorks(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: false},
	}}

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatalf("From a stopped container = %v, want the copy made", err)
	}

	copied := transfer.Files.(*files).from
	if len(copied) != 1 || copied[0][0] != "/var/www/html/vendor/." {
		t.Fatalf("copied %v, want vendor out of the stopped container", copied)
	}
}

// Copying in still needs it running: the files are given back to PHP's user with a chown run
// inside the container.
func TestCopyingIntoAStoppedContainerIsStillRefused(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: false},
	}}
	transfer.Binary = "hm"

	err := transfer.Into(project, []string{"vendor"}, "app:app")

	var refused core.Refusal
	if !errors.As(err, &refused) || refused.Kind != "service_not_running" || refused.Code != 5 ||
		refused.Message != "Service 'phpfpm' is not running" || refused.Hint != "hm start phpfpm" {
		t.Fatalf("Into a stopped container = %#v, want the service_not_running refusal", err)
	}

	if into := transfer.Files.(*files).into; len(into) != 0 {
		t.Fatalf("copied %v into a stopped container", into)
	}
}

// A project with no php container at all has nothing to copy out of.
func TestCopyingOutWithNoContainerIsRefused(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{}

	err := transfer.From(project, []string{"vendor"})

	var refused core.Refusal
	if !errors.As(err, &refused) || refused.Kind != "service_not_running" {
		t.Fatalf("From with no container = %#v, want the service_not_running refusal", err)
	}
}

// meteredFiles is a transfer that can say how far it has got, and keeps which kind of copy it was
// asked for.
type meteredFiles struct {
	plain, metered int

	// broken makes every metered copy fail after carrying a part of what it was asked for
	broken error
}

func (f *meteredFiles) Into(string, string, string) error { f.plain++; return nil }
func (f *meteredFiles) From(string, string, string) error { f.plain++; return nil }

func (f *meteredFiles) IntoMetered(_, _, _ string, meter ports.CopyMeter) error {
	f.metered++
	meter(11, 2)

	return f.broken
}

func (f *meteredFiles) FromMetered(_, _, _ string, meter ports.CopyMeter) error {
	f.metered++
	meter(7, 1)

	return f.broken
}

// watching keeps the totals each copy was shown with, what it was told, and whether it was ended.
type watching struct {
	totals       []int64
	bytes, files int64
	ended        int
	failed       bool
}

func (w *watching) watch(total int64) (ports.CopyMeter, func(bool)) {
	w.totals = append(w.totals, total)

	return func(bytes, files int64) { w.bytes, w.files = bytes, files },
		func(failed bool) { w.ended++; w.failed = failed }
}

// watched is transferencia with a vendor of 11 bytes in two files, a metered transfer and a watch.
func watched(t *testing.T, comun *runner) (Transfer, core.Project, *meteredFiles, *watching) {
	t.Helper()

	transfer, project := transferencia(t, comun, &runner{})

	for name, content := range map[string]string{"one.php": "hello", "two.php": "world!"} {
		if err := os.WriteFile(filepath.Join(project.Root, "vendor", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, watch := &meteredFiles{}, &watching{}
	transfer.Files, transfer.Watch = files, watch.watch

	return transfer, project, files, watch
}

func TestCopyingInIsWatchedWithTheTotalMeasuredOnThisMachine(t *testing.T) {
	transfer, project, files, watch := watched(t, &runner{})

	if err := transfer.Into(project, []string{"vendor"}, "app:app"); err != nil {
		t.Fatal(err)
	}

	if files.metered != 1 || files.plain != 0 {
		t.Fatalf("%d metered and %d plain copies, want one metered", files.metered, files.plain)
	}

	if len(watch.totals) != 1 || watch.totals[0] != 11 || watch.ended != 1 {
		t.Fatalf("watched totals %v, ended %d; want [11] ended once", watch.totals, watch.ended)
	}

	if watch.bytes != 11 || watch.files != 2 {
		t.Fatalf("the watch was told %d bytes, %d files; want what the copy reported", watch.bytes, watch.files)
	}
}

func TestCopyingEverythingInIsWatchedWithTheTotalMeasuredOnThisMachine(t *testing.T) {
	transfer, project, files, watch := watched(t, &runner{})

	if err := transfer.All(project, ""); err != nil {
		t.Fatal(err)
	}

	if files.metered != 1 || len(watch.totals) != 1 || watch.totals[0] != 11 || watch.ended != 1 {
		t.Fatalf("metered %d, totals %v, ended %d; want one copy watched with 11", files.metered,
			watch.totals, watch.ended)
	}
}

// transfer-db's dump is the largest single file this tool copies, and it is copied in like any
// other: watched, against the size of the file on this machine.
func TestADumpCopiedIntoAServiceIsWatchedWithItsSize(t *testing.T) {
	transfer, project, files, watch := watched(t, &runner{})
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "db1", Project: "tienda", ComposeService: "db", Running: true},
	}}

	dump := filepath.Join(t.TempDir(), "db.sql.gz")
	if err := os.WriteFile(dump, []byte("0123456789abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := transfer.IntoService(project, "db", dump, "/tmp"); err != nil {
		t.Fatal(err)
	}

	if files.metered != 1 || files.plain != 0 {
		t.Fatalf("%d metered and %d plain copies, want one metered", files.metered, files.plain)
	}

	if len(watch.totals) != 1 || watch.totals[0] != 13 || watch.ended != 1 || watch.failed {
		t.Fatalf("watched totals %v, ended %d, failed %v; want [13] ended once as done",
			watch.totals, watch.ended, watch.failed)
	}
}

func TestCopyingOutOfARunningContainerAsksDuForTheTotal(t *testing.T) {
	comun := &runner{answer: "675000000\t/var/www/html/vendor\n"}
	transfer, project, files, watch := watched(t, comun)

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(comun.command, " "); got != "du -sb /var/www/html/vendor" {
		t.Fatalf("asked the container %q, want du of the path without the trailing /.", got)
	}

	if files.metered != 1 || len(watch.totals) != 1 || watch.totals[0] != 675000000 || watch.ended != 1 {
		t.Fatalf("metered %d, totals %v, ended %d; want one copy watched with du's total",
			files.metered, watch.totals, watch.ended)
	}
}

// Out of a stopped container nothing can be asked inside it: a counter, with no total.
func TestCopyingOutOfAStoppedContainerIsWatchedWithoutATotal(t *testing.T) {
	comun := &runner{answer: "675000000\t/var/www/html/vendor\n"}
	transfer, project, files, watch := watched(t, comun)
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: false},
	}}

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatal(err)
	}

	if comun.command != nil {
		t.Fatalf("ran %q inside a stopped container", comun.command)
	}

	if files.metered != 1 || len(watch.totals) != 1 || watch.totals[0] != 0 {
		t.Fatalf("metered %d, totals %v; want one copy watched with no total", files.metered, watch.totals)
	}
}

// du is a guess at a total, and a guess that fails is no total rather than a failed copy.
func TestADuThatFailsLeavesTheCopyWithoutATotal(t *testing.T) {
	tests := []struct {
		name  string
		comun *runner
	}{
		{"du exits with an error", &runner{status: 1, answer: "675000000\t/var/www/html/vendor\n"}},
		{"du answers something else", &runner{answer: "du: not found\n"}},
		{"du answers nothing", &runner{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer, project, files, watch := watched(t, tt.comun)

			if err := transfer.From(project, []string{"vendor"}); err != nil {
				t.Fatalf("From = %v, want the copy made anyway", err)
			}

			if files.metered != 1 || len(watch.totals) != 1 || watch.totals[0] != 0 {
				t.Fatalf("metered %d, totals %v; want the copy watched with no total", files.metered, watch.totals)
			}
		})
	}
}

// Nobody watching is the copy as it always was: the plain transfer, nothing measured, nothing
// asked inside the container.
func TestWithoutAWatchNothingIsMeasuredOrAsked(t *testing.T) {
	comun := &runner{answer: "675000000\t/var/www/html/vendor\n"}
	transfer, project, files, _ := watched(t, comun)
	transfer.Watch = nil

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatal(err)
	}

	if comun.command != nil {
		t.Fatalf("ran %q inside the container with nobody watching", comun.command)
	}

	if err := transfer.Into(project, []string{"vendor"}, ""); err != nil {
		t.Fatal(err)
	}

	if files.metered != 0 || files.plain != 2 {
		t.Fatalf("%d metered and %d plain copies, want two plain", files.metered, files.plain)
	}
}

// A copy that fails partway is ended as failed, in and out, so that what is drawn does not read
// as a finished copy just before the error.
func TestAFailedCopyIsEndedAsFailed(t *testing.T) {
	for name, copy := range map[string]func(Transfer, core.Project) error{
		"in":  func(t Transfer, p core.Project) error { return t.Into(p, []string{"vendor"}, "app:app") },
		"out": func(t Transfer, p core.Project) error { return t.From(p, []string{"vendor"}) },
	} {
		t.Run(name, func(t *testing.T) {
			transfer, project, files, watch := watched(t, &runner{})
			files.broken = errors.New("the daemon closed the stream")

			if err := copy(transfer, project); !errors.Is(err, files.broken) {
				t.Fatalf("copy = %v, want the copy's own error", err)
			}

			if watch.ended != 1 || !watch.failed {
				t.Fatalf("ended %d, failed %v; want ended once, as failed", watch.ended, watch.failed)
			}
		})
	}
}

// One that works is ended as done.
func TestACopyThatWorksIsEndedAsDone(t *testing.T) {
	transfer, project, _, watch := watched(t, &runner{})

	if err := transfer.Into(project, []string{"vendor"}, "app:app"); err != nil {
		t.Fatal(err)
	}

	if watch.ended != 1 || watch.failed {
		t.Fatalf("ended %d, failed %v; want ended once, not failed", watch.ended, watch.failed)
	}
}

// Copying out takes a stopped container only when there is no running one: an orphaned stopped
// phpfpm listed before the live one must not be the one read.
func TestCopyingOutPrefersTheRunningContainer(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{containers: []core.Container{
		{ID: "old", Project: "tienda", ComposeService: "phpfpm", Running: false},
		{ID: "live", Project: "tienda", ComposeService: "phpfpm", Running: true},
	}}

	recorder := &containerFiles{}
	transfer.Files = recorder

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatal(err)
	}

	if len(recorder.from) != 1 || recorder.from[0] != "live" {
		t.Fatalf("copied out of %v, want the running container", recorder.from)
	}
}

// With no php container at all, the refusal says it is not there rather than that it is stopped:
// the shared resolution's answer, which names the service like every other.
func TestCopyingOutOfNoContainerSaysItIsNotThere(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{}
	transfer.Binary = "hm"

	err := transfer.From(project, []string{"vendor"})

	var refused core.Refusal
	if !errors.As(err, &refused) || refused.Code != 5 ||
		refused.Message != "This project has no phpfpm container" || refused.Hint != "hm start phpfpm" {
		t.Fatalf("From with no container = %#v, want the refusal saying there is none", err)
	}
}

// containerFiles records which container each copy out read.
type containerFiles struct{ from []string }

func (f *containerFiles) Into(string, string, string) error { return nil }
func (f *containerFiles) From(container, _, _ string) error {
	f.from = append(f.from, container)

	return nil
}
