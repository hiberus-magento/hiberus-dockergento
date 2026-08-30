package cli

import (
	"runtime"
	"testing"
)

//
// On macOS those four subcommands do not simply run in the container: the dependencies are copied
// in, Composer runs, and the whole tree is copied back out over the host's — deleting the host's
// vendor directory on the way. That path is still the shell implementation's, and which
// invocations take it is the decision worth pinning down.
//

func TestTheInvocationsThatRewriteTheHostsTree(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("el baile del vendor sólo existe en macOS")
	}

	for _, subcomando := range []string{"install", "update", "require", "remove"} {
		if !mirrorsVendor([]string{subcomando}) {
			t.Errorf("`composer %s` reescribe el vendor del host", subcomando)
		}
	}
}

func TestEverythingElseRunsInTheContainer(t *testing.T) {
	for _, subcomando := range [][]string{{"show"}, {"dump-autoload"}, {"--version"}, {}} {
		if mirrorsVendor(subcomando) {
			t.Errorf("`composer %v` no toca el vendor del host", subcomando)
		}
	}
}

func TestOnlyTheFourWriteDependencies(t *testing.T) {
	// The same list decides the refusal in a worktree that reads somebody else's dependencies,
	// on either platform
	if !writesDependencies([]string{"require", "vendor/paquete"}) {
		t.Error("`require` escribe dependencias")
	}

	if writesDependencies([]string{"show"}) {
		t.Error("`show` no escribe nada")
	}

	if writesDependencies(nil) {
		t.Error("sin subcomando no se escribe nada")
	}
}
