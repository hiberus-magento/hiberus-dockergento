package app

import (
	"reflect"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// Composer's macOS dance copies vendor back out before the rest of the workdir, so that a copy
// that fails later in the tree does not leave the host's vendor, which it emptied, empty. The
// whole-workdir copy that follows does not carry vendor a second time.
//

// skippingFiles is a transfer that can leave named entries at the top of a copied tree unwritten,
// and keeps every copy out it was asked for with what it was asked to skip.
type skippingFiles struct {
	files

	skipped [][]string
	metered int
}

func (f *skippingFiles) FromSkipping(_, source, target string, skip []string, meter ports.CopyMeter) error {
	f.from = append(f.from, [2]string{source, target})
	f.skipped = append(f.skipped, skip)

	if meter != nil {
		f.metered++
		meter(3, 1)
	}

	return nil
}

func TestTheWorkdirCopiedAfterVendorDoesNotCarryVendorAgain(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	recorder := &skippingFiles{}
	transfer.Files = recorder

	if err := transfer.From(project, []string{"vendor", "."}); err != nil {
		t.Fatal(err)
	}

	want := [][2]string{{"/var/www/html/vendor/.", project.Root + "/vendor"}, {"/var/www/html/.", project.Root}}
	if !reflect.DeepEqual(recorder.from, want) {
		t.Fatalf("copied %v, want vendor first, then the workdir", recorder.from)
	}

	if !reflect.DeepEqual(recorder.skipped, [][]string{{"vendor"}}) {
		t.Fatalf("skipped %v, want only the workdir copy to leave vendor out", recorder.skipped)
	}
}

func TestAWatchedWorkdirCopyAfterVendorIsStillWatched(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	recorder, watch := &skippingFiles{}, &watching{}
	transfer.Files, transfer.Watch = recorder, watch.watch

	if err := transfer.From(project, []string{"vendor", "."}); err != nil {
		t.Fatal(err)
	}

	// The vendor copy goes plain here, this fake having no metered copy: only the workdir is watched
	if recorder.metered != 1 || watch.ended != 1 || watch.bytes != 3 {
		t.Fatalf("metered %d skipping copies, ended %d, told %d bytes; want 1, 1 and 3",
			recorder.metered, watch.ended, watch.bytes)
	}
}

// A transfer that cannot skip copies the workdir whole: vendor twice, which is slower and still
// right.
func TestATransferThatCannotSkipCopiesTheWorkdirWhole(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})

	if err := transfer.From(project, []string{"vendor", "."}); err != nil {
		t.Fatal(err)
	}

	copied := transfer.Files.(*files).from
	if len(copied) != 2 || copied[1][0] != "/var/www/html/." {
		t.Fatalf("copied %v, want vendor and then the whole workdir", copied)
	}
}

// Asked on its own, the workdir copy skips nothing: what the command line asks is what it gets.
func TestTheWorkdirCopiedOnItsOwnSkipsNothing(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	recorder := &skippingFiles{}
	transfer.Files = recorder

	if err := transfer.From(project, []string{"."}); err != nil {
		t.Fatal(err)
	}

	if len(recorder.from) != 1 || len(recorder.skipped) != 0 {
		t.Fatalf("copied %v skipping %v, want one plain copy", recorder.from, recorder.skipped)
	}
}

// A nested path copied before the workdir (app/code, then .) leaves nothing out of the workdir:
// skipping the whole top entry (app) would lose app/etc and the rest of app, which the first copy
// never carried. Only a top entry copied whole is left out.
func TestANestedPathCopiedFirstLeavesNothingOutOfTheWorkdir(t *testing.T) {
	transfer, project := transferencia(t, &runner{}, &runner{})
	recorder := &skippingFiles{}
	transfer.Files = recorder

	if err := transfer.From(project, []string{"app/code", "vendor/acme", "."}); err != nil {
		t.Fatal(err)
	}

	// With nothing to leave out the copy goes plain, so no skipping copy is asked at all
	for _, skip := range recorder.skipped {
		if len(skip) != 0 {
			t.Fatalf("skipped %v, want the workdir copied whole after nested paths", recorder.skipped)
		}
	}

	if len(recorder.from) != 3 || recorder.from[2][0] != "/var/www/html/." {
		t.Fatalf("copied %v, want the two nested paths and then the whole workdir", recorder.from)
	}
}
