package cli

import (
	"os"
	"testing"
)

// /dev/null is a character device and not a terminal, and the difference is not academic: the
// output format is chosen by this answer, and so is whether `exec` asks Docker for a pseudo
// terminal — which fails outright when there is none.
func TestDevNullIsNotATerminal(t *testing.T) {
	nowhere, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no se puede abrir %s: %v", os.DevNull, err)
	}
	defer nowhere.Close()

	if isTerminal(nowhere) {
		t.Fatal("isTerminal(/dev/null) = true, want false")
	}
}

func TestAPlainFileIsNotATerminal(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "salida")
	if err != nil {
		t.Fatalf("creating the file = %v, want no error", err)
	}
	defer file.Close()

	if isTerminal(file) {
		t.Fatal("isTerminal(an ordinary file) = true, want false")
	}
}

func TestSomethingThatIsNotAFileIsNotATerminal(t *testing.T) {
	if isTerminal(nil) {
		t.Fatal("isTerminal(not a file) = true, want false")
	}
}
