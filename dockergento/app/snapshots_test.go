package app

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// copias builds the use case over a database that answers with what the fake runner was given.
func copias(t *testing.T, corredor *runner) (Snapshots, core.Project) {
	t.Helper()

	project := core.Project{Name: "tienda", Root: t.TempDir()}

	return Snapshots{
		Database: baseDeDatos([]core.Container{contenedorDeDatos("tienda", true)}, corredor),
		State:    estado{},
		Dir:      t.TempDir(),
		Binary:   "hm",
		Errors:   io.Discard,
	}, project
}

// estado is the record of whether the data has been anonymised, which a restore clears.
type estado struct{ cleared bool }

func (estado) Anonymisation(string) (string, string) { return "unknown", "" }
func (estado) Record(string, string) error           { return nil }
func (estado) Clear(string) error                    { return nil }

// What the dumper writes to its error stream must not end up inside the copy.
//
// A dump is the output, so a warning in the middle of it — "using a password on the command line
// is insecure" is the usual one — is a copy that fails to restore at the line it appears on. It
// would be found the day somebody needed the copy, which is the worst day to find it.
func TestAComplaintDoesNotEndUpInsideTheCopy(t *testing.T) {
	corredor := &runner{answer: "CREATE TABLE pedidos (id INT);\n", complaint: "Warning: insecure\n"}
	snapshots, project := copias(t, corredor)

	taken, err := snapshots.Take(project, "antes", false)
	if err != nil {
		t.Fatalf("Take = %v, want no error", err)
	}

	file, err := os.Open(taken.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	contents, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("reading the copy = %v, want a gzip file", err)
	}

	written, err := io.ReadAll(contents)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(written), "Warning") {
		t.Fatalf("copy = %q, want the dumper's complaint outside it", written)
	}

	if !strings.Contains(string(written), "CREATE TABLE") {
		t.Fatalf("copy = %q, want the dump in it", written)
	}

	// The header says what it is, which is what makes a file found a year later readable
	if !strings.Contains(string(written), "-- hm snapshot: antes") {
		t.Fatalf("copy = %q, want a header saying what it is", written)
	}
}

// A dump that failed leaves no snapshot. What it must not leave is a file that looks like one:
// the half-written copy is deleted, and nothing is renamed into place.
func TestAFailedCopyLeavesNothingBehind(t *testing.T) {
	corredor := &runner{answer: "partial", complaint: "died", status: 2}
	snapshots, project := copias(t, corredor)

	if _, err := snapshots.Take(project, "antes", false); err == nil {
		t.Fatal("Take(a dump that failed) = nil, want an error")
	}

	found, err := filepath.Glob(filepath.Join(snapshots.Dir, "*", "*"))
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 0 {
		t.Fatalf("files left behind = %v, want none", found)
	}
}
