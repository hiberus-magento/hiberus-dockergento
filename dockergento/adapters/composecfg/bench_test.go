package composecfg

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The number that decides whether reading with the library is worth a dependency: what the same
// answer costs each way. Fourteen call sites in the shell implementation pay the second one.

func BenchmarkLibrary(b *testing.B) {
	root := labFor(b)

	for b.Loop() {
		if _, err := (Loader{}).Load(root, "lab", []string{"docker-compose.yml", "docker-compose.dev.mac.yml"}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTheCommand(b *testing.B) {
	root := labFor(b)

	for b.Loop() {
		command := exec.Command("docker", "compose",
			"-f", filepath.Join(root, "docker-compose.yml"),
			"-f", filepath.Join(root, "docker-compose.dev.mac.yml"),
			"-p", "lab", "config", "--format", "json")
		command.Dir = root

		if _, err := command.Output(); err != nil {
			b.Skip("docker compose no está disponible aquí")
		}
	}
}
