package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// On macOS some of the project is bind mounted into the container (app, config, composer.json…):
// those are the host's own files, seen from inside. Copying the workdir back out wrote them onto
// themselves while the daemon was reading them, and the copy died halfway with "unexpected EOF".
//

// boundTransfer is a transfer whose php container has the project's bind mounts of a macOS
// environment, a nested one included.
func boundTransfer(t *testing.T) (Transfer, core.Project, *skippingFiles) {
	t.Helper()

	transfer, project := transferencia(t, &runner{}, &runner{})
	transfer.Engine = &engine{containers: []core.Container{{
		ID: "php1", Project: "tienda", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{
			{Type: "volume", Destination: "/var/www/html"},
			{Type: "bind", Destination: "/var/www/html/app"},
			{Type: "bind", Destination: "/var/www/html/composer.json"},
			{Type: "bind", Destination: "/var/www/html/dev/tests/unit"},
			{Type: "bind", Destination: "/etc/ssl/local"},
		},
	}}}

	recorder := &skippingFiles{}
	transfer.Files = recorder

	return transfer, project, recorder
}

func TestCopyingTheWorkdirOutLeavesTheBindMountsAlone(t *testing.T) {
	transfer, project, recorder := boundTransfer(t)

	if err := transfer.From(project, []string{"vendor", "."}); err != nil {
		t.Fatal(err)
	}

	// vendor, copied first, and every bind mount under the workdir; nothing outside it
	want := [][]string{{"vendor", "app", "composer.json", "dev/tests/unit"}}
	if !reflect.DeepEqual(recorder.skipped, want) {
		t.Fatalf("skipped %v, want %v", recorder.skipped, want)
	}
}

// Asked for by name, a bind mount is refused, as copying it in is: it is already the host's file.
func TestCopyingABindMountOutIsRefused(t *testing.T) {
	transfer, project, recorder := boundTransfer(t)

	err := transfer.From(project, []string{"app"})

	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Kind != "path_is_bound" {
		t.Fatalf("From(app) = %v, want the path_is_bound refusal", err)
	}

	if len(recorder.from) != 0 {
		t.Fatalf("copied %v, want nothing copied", recorder.from)
	}
}

// A path that is not bound copies as before, with nothing to leave out.
func TestCopyingAnUnboundPathOutIsUnchanged(t *testing.T) {
	transfer, project, recorder := boundTransfer(t)

	if err := transfer.From(project, []string{"vendor"}); err != nil {
		t.Fatal(err)
	}

	if len(recorder.from) != 1 || len(recorder.skipped) != 0 {
		t.Fatalf("copied %v skipping %v, want one plain copy of vendor", recorder.from, recorder.skipped)
	}
}
