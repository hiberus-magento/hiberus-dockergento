package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What setup asks of the engine. The handler (setup.go:20) reads the whole instruction with
// core.ParseSetup — pinned in core/setup_test.go — asks for one Setup, and then resolves the project
// it created to say where it answers. What a setup is — the questions, the compose files, the
// install steps — is app.Setup's, and is tested there. Dir is the working directory.
//

func TestWhatSetupAsks(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Domain: "shop.test", MagentoDir: "src"}
	cwd := here()

	args := []string{"-p", "shop", "--domain=shop.test", "-r", "src", "-f"}
	if code := setup(args, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("setup %v = %d, want exitOK", args, code)
	}

	options, err := core.ParseSetup(args, binaryName())
	if err != nil {
		t.Fatalf("ParseSetup(%v) = %v", args, err)
	}

	// answering(t) sets HM_NON_INTERACTIVE, so nothing may be asked
	want := []call{
		{Method: "Setup", Dir: cwd, SetupOptions: options, Interactive: false},
		{Method: "Resolve", Dir: cwd},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}

	if !options.Force || options.ProjectName != "shop" {
		t.Errorf("the instruction was not read whole: %+v", options)
	}
}

// TestSetupMayAskWithSomebodyThere: with nobody saying otherwise, setup may ask its questions.
func TestSetupMayAskWithSomebodyThere(t *testing.T) {
	fake := answering(t)
	t.Setenv("HM_NON_INTERACTIVE", "")
	fake.project = core.Project{Name: "shop", Domain: "shop.test"}

	if code := setup(nil, io.Discard, io.Discard, false); code != exitOK {
		t.Fatalf("setup = %d, want exitOK", code)
	}

	if got := fake.calls[0]; got.Method != "Setup" || !got.Interactive {
		t.Fatalf("first call = %+v, want an interactive Setup", got)
	}
}

// TestSetupRefusesAnInstructionItCannotRead: a malformed instruction creates nothing.
func TestSetupRefusesAnInstructionItCannotRead(t *testing.T) {
	fake := answering(t)

	if code := setup([]string{"--nonsense"}, io.Discard, io.Discard, true); code != exitUsage {
		t.Fatalf("setup --nonsense = %d, want exitUsage", code)
	}

	if len(fake.calls) != 0 {
		t.Errorf("asked the engine %v, want nothing", fake.calls)
	}
}

// TestSetupSaysWhereTheProjectAnswers: under --json, the project it created, its domain and where
// its code is; in text, the address to open.
func TestSetupSaysWhereTheProjectAnswers(t *testing.T) {
	fake := answering(t)
	fake.project = core.Project{Name: "shop", Domain: "shop.test", MagentoDir: "src"}

	var document bytes.Buffer
	if code := setup(nil, &document, io.Discard, true); code != exitOK {
		t.Fatalf("setup --json = %d, want exitOK", code)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(document.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, document.String())
	}

	want := map[string]any{"project": "shop", "domain": "shop.test", "magento_dir": "src"}
	if diff := cmp.Diff(want, envelope.Data); diff != "" {
		t.Errorf("data (-want +got):\n%s", diff)
	}

	var text bytes.Buffer
	fake.calls = nil
	if code := setup(nil, &text, io.Discard, false); code != exitOK {
		t.Fatalf("setup = %d, want exitOK", code)
	}

	if !strings.Contains(text.String(), "https://shop.test/") {
		t.Errorf("stdout = %q, want the address to open", text.String())
	}
}

// TestAFailedSetupIsNotASuccess: an error from the engine stops there, with nothing resolved.
func TestAFailedSetupIsNotASuccess(t *testing.T) {
	fake := answering(t)
	fake.outcomes = []outcome{{err: core.Refusal{Kind: "setup_failed", Code: 17, Message: "failed"}}}

	if code := setup(nil, io.Discard, io.Discard, true); code != 17 {
		t.Fatalf("setup with a failing engine = %d, want the refusal's own 17", code)
	}

	if len(fake.calls) != 1 {
		t.Errorf("asked the engine %v, want only the failed Setup", fake.calls)
	}

}
