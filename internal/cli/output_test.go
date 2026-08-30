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
		t.Fatal("/dev/null no es un terminal, por mucho que sea un dispositivo de caracteres")
	}
}

func TestAPlainFileIsNotATerminal(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "salida")
	if err != nil {
		t.Fatalf("no se pudo crear el fichero: %v", err)
	}
	defer file.Close()

	if isTerminal(file) {
		t.Fatal("un fichero corriente tampoco lo es")
	}
}

func TestSomethingThatIsNotAFileIsNotATerminal(t *testing.T) {
	if isTerminal(nil) {
		t.Fatal("sin fichero no hay terminal")
	}
}
