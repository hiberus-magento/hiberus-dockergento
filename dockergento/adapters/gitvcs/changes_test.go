package gitvcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func write(t *testing.T, dir, name string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestChangedSinceIsWhatTheBranchChangedSinceItLeftTheBase: the PHP files added or modified on the
// branch, and not what the base did after the branch left it, nor what the branch deleted.
func TestChangedSinceIsWhatTheBranchChangedSinceItLeftTheBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	repository := t.TempDir()
	git(t, repository, "init", "-q", "-b", "main", ".")
	write(t, repository, "app/code/Old.php")
	write(t, repository, "app/code/Gone.php")
	git(t, repository, "add", "-A")
	git(t, repository, "commit", "-qm", "base")

	git(t, repository, "checkout", "-q", "-b", "feature")
	write(t, repository, "app/code/New.php")
	write(t, repository, "app/code/notes.txt")
	if err := os.WriteFile(filepath.Join(repository, "app/code/Old.php"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repository, "rm", "-q", "app/code/Gone.php")
	git(t, repository, "add", "-A")
	git(t, repository, "commit", "-qm", "feature")

	git(t, repository, "checkout", "-q", "main")
	write(t, repository, "app/code/OnMain.php")
	git(t, repository, "add", "-A")
	git(t, repository, "commit", "-qm", "main moved on")
	git(t, repository, "checkout", "-q", "feature")

	if !(Git{}).Exists(repository, "main") || (Git{}).Exists(repository, "master") {
		t.Fatal("Exists does not tell main, which is there, from master, which is not")
	}

	got := Git{}.ChangedSince(repository, "main", "*.php")
	if want := []string{"app/code/New.php", "app/code/Old.php"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedSince = %v, want %v", got, want)
	}
}

func TestChangedSinceOutsideARepositoryIsNothing(t *testing.T) {
	if got := (Git{}).ChangedSince(t.TempDir(), "main", "*.php"); len(got) != 0 {
		t.Fatalf("ChangedSince outside a repository = %v, want nothing", got)
	}
}
