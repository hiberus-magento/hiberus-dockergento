package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// transfer-media copies a remote store's pub/media into this one's with rsync over SSH, without
// overwriting what is already here and leaving the caches behind. Never a real rsync here: the
// runner and the lookup are replaced, and what they were asked is what is checked.
//

// mediaLab is a project directory with pub/media, rsync on the PATH and a recording runner.
func mediaLab(t *testing.T) *[][]string {
	t.Helper()

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: here()}

	if err := os.MkdirAll(filepath.Join(here(), "pub", "media"), 0o755); err != nil {
		t.Fatal(err)
	}

	ran := &[][]string{}

	previousRun, previousLook := runRsync, lookRsync
	t.Cleanup(func() { runRsync, lookRsync = previousRun, previousLook })

	runRsync = func(args []string) error {
		*ran = append(*ran, args)

		return nil
	}
	lookRsync = func() error { return nil }

	return ran
}

func TestTransferMediaRsyncsTheRemoteMediaIntoPub(t *testing.T) {
	ran := mediaLab(t)

	code := transferMedia([]string{"--ssh-user=abc", "--ssh-host=ssh.example.cloud", "--ssh-path=/app/pub/media"},
		io.Discard, io.Discard, false)
	if code != exitOK {
		t.Fatalf("transfer-media = %d, want exitOK", code)
	}

	want := "-az --ignore-existing --exclude **/cache abc@ssh.example.cloud:/app/pub/media ./pub"
	if len(*ran) != 1 || strings.Join((*ran)[0], " ") != want {
		t.Fatalf("rsync ran with %v, want [%s]", *ran, want)
	}
}

// TestTheMagentoCloudHostAndPathAreTheDefaults: only the user has no default.
func TestTheMagentoCloudHostAndPathAreTheDefaults(t *testing.T) {
	ran := mediaLab(t)

	if code := transferMedia([]string{"--ssh-user=abc"}, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("transfer-media = %d, want exitOK", code)
	}

	if got := strings.Join((*ran)[0], " "); !strings.Contains(got, "abc@ssh.eu-3.magento.cloud:/app/pub/media") {
		t.Fatalf("rsync ran with %q, want the Magento Cloud defaults", got)
	}
}

func TestTransferMediaRefusesWithoutAUser(t *testing.T) {
	ran := mediaLab(t)

	if code := transferMedia(nil, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("transfer-media with no user = %d, want exitUsage", code)
	}

	if len(*ran) != 0 {
		t.Errorf("rsync ran: %v", *ran)
	}
}

func TestTransferMediaNeedsRsyncAndAMediaDirectory(t *testing.T) {
	t.Run("no rsync", func(t *testing.T) {
		ran := mediaLab(t)
		lookRsync = func() error { return errors.New("not found") }

		if code := transferMedia([]string{"--ssh-user=abc"}, io.Discard, io.Discard, true); code != exitError {
			t.Fatalf("transfer-media without rsync = %d, want exitError", code)
		}

		if len(*ran) != 0 {
			t.Errorf("rsync ran: %v", *ran)
		}
	})

	t.Run("no pub/media here", func(t *testing.T) {
		ran := mediaLab(t)
		if err := os.RemoveAll(filepath.Join(here(), "pub")); err != nil {
			t.Fatal(err)
		}

		if code := transferMedia([]string{"--ssh-user=abc"}, io.Discard, io.Discard, true); code != exitError {
			t.Fatalf("transfer-media with no pub/media = %d, want exitError", code)
		}

		if len(*ran) != 0 {
			t.Errorf("rsync ran: %v", *ran)
		}
	})
}

// TestAFailedRsyncIsAFailure: an interrupted copy is not "All media content transferred".
func TestAFailedRsyncIsAFailure(t *testing.T) {
	mediaLab(t)
	runRsync = func([]string) error { return errors.New("exit status 23") }

	if code := transferMedia([]string{"--ssh-user=abc"}, io.Discard, io.Discard, true); code == exitOK {
		t.Fatal("transfer-media with rsync failing = exitOK, want a failure")
	}
}

// TestTransferMediaRefusesAnOptionItDoesNotKnow: the shell ignored it; ignoring it is believing
// it did something.
func TestTransferMediaRefusesAnOptionItDoesNotKnow(t *testing.T) {
	ran := mediaLab(t)

	if code := transferMedia([]string{"--ssh-port=22"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("transfer-media --ssh-port=22 = %d, want exitUsage", code)
	}

	if len(*ran) != 0 {
		t.Errorf("rsync ran: %v", *ran)
	}
}

// TestTheRouterAnswersForTransferMedia: it reaches its handler, not the shell.
func TestTheRouterAnswersForTransferMedia(t *testing.T) {
	ran := mediaLab(t)

	Run([]string{"transfer-media", "--ssh-user=abc"}, io.Discard, io.Discard)

	if len(*ran) != 1 {
		t.Fatalf("transfer-media did not reach its handler: rsync ran %v", *ran)
	}
}
