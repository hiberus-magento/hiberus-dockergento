package dockergento

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// Moving the tool between releases. Never the network here: every engine gets a fake source, and
// TestMain replaces the default one so that an engine built without it fails instead of reaching
// GitHub. The executable replaced is always a temporary copy, never the test binary.
//

func TestMain(m *testing.M) {
	newInstaller = func() installer {
		return installer{
			source:     &fakeReleases{err: errors.New("no network in tests")},
			executable: func() (string, error) { return "", errors.New("no executable in tests") },
			goos:       "linux", goarch: "amd64",
		}
	}

	os.Exit(m.Run())
}

// fakeReleases lists releases and serves assets by URL, recording what it was asked.
type fakeReleases struct {
	listed []release
	files  map[string]string
	err    error

	asked []string
}

func (f *fakeReleases) Releases() ([]release, error) {
	f.asked = append(f.asked, "list")

	return f.listed, f.err
}

func (f *fakeReleases) Download(url string, watch ports.CopyWatch) ([]byte, error) {
	f.asked = append(f.asked, url)

	content, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("%s answered 404 Not Found", url)
	}

	// The way a server answers: the size first, then the body, in two halves
	if watch != nil {
		meter, end := watch(int64(len(content)))
		meter(int64(len(content)/2), 1)
		meter(int64(len(content)), 1)
		end(false)
	}

	return []byte(content), nil
}

func (f *fakeReleases) downloaded() int { return len(f.asked) - 1 }

func sum(content string) string {
	digest := sha256.Sum256([]byte(content))

	return hex.EncodeToString(digest[:])
}

// publish adds a release whose linux/amd64 binary is "#!/bin/sh\n# <tag>\n", with its checksums.
func (f *fakeReleases) publish(tag string, preRelease bool) {
	if f.files == nil {
		f.files = map[string]string{}
	}

	base := "https://example.invalid/" + tag + "/"
	binary := "#!/bin/sh\n# " + tag + "\n"

	f.files[base+"hm_linux_amd64"] = binary
	f.files[base+"checksums.txt"] = sum(binary) + "  hm_linux_amd64\n" + sum("other") + "  hm_darwin_arm64\n"

	f.listed = append(f.listed, release{Tag: tag, PreRelease: preRelease, Assets: map[string]string{
		"hm_linux_amd64": base + "hm_linux_amd64",
		"checksums.txt":  base + "checksums.txt",
	}})
}

// installed is an executable in a directory of its own, holding "old", and an engine that replaces
// it with releases from source.
func installed(t *testing.T, source *fakeReleases) (*Engine, string) {
	t.Helper()

	// Resolved, because the replacement resolves the executable's links and a temporary directory
	// can sit behind one (/var is /private/var on macOS)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "hm")

	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: t.TempDir(), Assets: fstest.MapFS{}})
	engine.install = &installer{
		source:     source,
		executable: func() (string, error) { return target, nil },
		goos:       "linux", goarch: "amd64",
	}

	return engine, target
}

func contentOf(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

// noLeftovers: a replacement, done or not, leaves only the executable in its directory.
func noLeftovers(t *testing.T, target string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("the directory holds %v, want only hm", names)
	}
}

func refusalOf(t *testing.T, err error) core.Refusal {
	t.Helper()

	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a refusal", err)
	}

	return refusal
}

// ------------------------------------------------------------------ update

func TestANewerReleaseReplacesTheRunningExecutable(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.2.0-rc.1", true)
	source.publish("2.1.0", false)
	source.publish("2.0.0", false)
	source.publish("1.6.0", false)
	engine, target := installed(t, source)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated != (core.Updated{Updated: true, From: "2.0.0", To: "2.1.0", Latest: "2.1.0", Path: target}) {
		t.Fatalf("updated = %+v", updated)
	}

	if got := contentOf(t, target); got != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("the executable holds %q, want the binary of 2.1.0", got)
	}

	if info, _ := os.Stat(target); info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}

	noLeftovers(t, target)
}

func TestUpdateWithPreReleasesTakesTheNewestOfAll(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.publish("2.2.0-rc.1", true)
	engine, target := installed(t, source)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.1.0", PreReleases: true})
	if err != nil || updated.To != "2.2.0-rc.1" || contentOf(t, target) != "#!/bin/sh\n# 2.2.0-rc.1\n" {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}
}

func TestAnUpToDateBinaryIsLeftAlone(t *testing.T) {
	for _, running := range []string{"2.1.0", "2.2.0-rc.1"} {
		source := &fakeReleases{}
		source.publish("2.1.0", false)
		engine, target := installed(t, source)

		updated, err := engine.Update(core.UpdateOptions{Running: running})
		if err != nil {
			t.Fatalf("Update from %s: %v", running, err)
		}

		if updated != (core.Updated{From: running, To: running, Latest: "2.1.0", Path: target}) {
			t.Fatalf("updated from %s = %+v", running, updated)
		}

		if contentOf(t, target) != "old" || source.downloaded() != 0 {
			t.Fatalf("from %s the executable was touched or something downloaded: %v", running, source.asked)
		}
	}
}

func TestWithNoReleaseOfTheGoToolThereIsNothingToUpdateTo(t *testing.T) {
	source := &fakeReleases{}
	source.publish("1.6.0", false)
	source.publish("2.0.0-rc.1", true)
	engine, target := installed(t, source)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0-rc.1"})
	if err != nil || updated.Updated || updated.Latest != "" || contentOf(t, target) != "old" {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}
}

func TestAChecksumMismatchKeepsTheOldBinary(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.files["https://example.invalid/2.1.0/hm_linux_amd64"] = "tampered"
	engine, target := installed(t, source)

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "download_failed" || refusal.Code != 1 ||
		!strings.Contains(refusal.Message, "does not match its checksum") ||
		!strings.Contains(refusal.Message, target+" was not changed") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" {
		t.Fatal("the old binary was replaced")
	}

	noLeftovers(t, target)
}

func TestAReleaseWithoutChecksumsIsNoBinary(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	delete(source.listed[0].Assets, "checksums.txt")
	engine, target := installed(t, source)

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	if refusal := refusalOf(t, err); refusal.Kind != "download_failed" ||
		!strings.Contains(refusal.Message, "publishes no checksums.txt") || contentOf(t, target) != "old" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestAnAssetTheChecksumsDoNotListIsNoBinary(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.files["https://example.invalid/2.1.0/checksums.txt"] = sum("other") + "  hm_darwin_arm64\n"
	engine, target := installed(t, source)

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	if refusal := refusalOf(t, err); !strings.Contains(refusal.Message, "do not list hm_linux_amd64") ||
		contentOf(t, target) != "old" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestAReleaseWithoutABinaryForThisPlatformIsSaid(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	engine.install.goos, engine.install.goarch = "darwin", "amd64"

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "asset_missing" || refusal.Code != 1 ||
		refusal.Message != "Release 2.1.0 publishes no binary for darwin/amd64; the running executable was not changed" ||
		!strings.Contains(refusal.Hint, "go build") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" || source.downloaded() != 0 {
		t.Fatalf("the executable was touched or something downloaded: %v", source.asked)
	}
}

func TestADirectoryThatCannotBeWrittenIsSaidBeforeDownloading(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}

	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)

	dir := filepath.Dir(target)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "install_dir_not_writable" || refusal.Code != 1 ||
		refusal.Message != "Cannot replace "+target+": "+dir+" is not writable by you, so 2.1.0 was not installed" ||
		!strings.Contains(refusal.Hint, installerOneLiner) || !strings.Contains(refusal.Hint, "HM_INSTALL_DIR") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" || source.downloaded() != 0 {
		t.Fatalf("the executable was touched or something downloaded: %v", source.asked)
	}
}

// installerOneLiner is the one-liner a refusal to replace the binary points at.
const installerOneLiner = "curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash"

// sudoer stands in for sudo: it records what it was asked to run, and answers with err.
type sudoer struct {
	asked [][]string
	err   error
}

func (s *sudoer) run(args ...string) error {
	s.asked = append(s.asked, args)

	return s.err
}

// atTerminal gives engine a person at a terminal who can become root through sudo, a home of its
// own, and somewhere to say things; it returns them.
func atTerminal(t *testing.T, engine *Engine) (*sudoer, string, *strings.Builder) {
	t.Helper()

	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sudo, notice := &sudoer{}, &strings.Builder{}

	engine.install.interactive = func() bool { return true }
	engine.install.home = func() (string, error) { return home, nil }
	engine.install.sudo = sudo.run
	engine.install.notice = notice

	return sudo, home, notice
}

// rootOwned makes the directory of target one the user cannot write, until the test ends.
func rootOwned(t *testing.T, target string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}

	dir := filepath.Dir(target)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestABinaryReachedThroughALinkIsReplacedWhereItReallyIs(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)

	link := filepath.Join(t.TempDir(), "hm")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	engine.install.executable = func() (string, error) { return link, nil }

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Path != target || contentOf(t, target) != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("updated = %+v, the real file holds %q", updated, contentOf(t, target))
	}

	if pointed, err := os.Readlink(link); err != nil || pointed != target {
		t.Fatalf("the link points at %q (%v), want %s", pointed, err, target)
	}

	noLeftovers(t, target)
}

func TestAWritableDirectoryAtATerminalIsReplacedInPlaceWithoutSudo(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	sudo, home, notice := atTerminal(t, engine)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})
	if err != nil || updated.Path != target || contentOf(t, target) != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}

	if len(sudo.asked) != 0 || notice.Len() != 0 {
		t.Fatalf("sudo was asked %v and the notice said %q, want neither", sudo.asked, notice.String())
	}

	if _, err := os.Stat(filepath.Join(home, ".hm")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("~/.hm was touched: %v", err)
	}
}

func TestADirectoryOnlyRootWritesMovesHmHomeWithSudoOnce(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	rootOwned(t, target)
	sudo, home, notice := atTerminal(t, engine)
	moved := filepath.Join(home, ".hm", "bin", "hm")

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated != (core.Updated{Updated: true, From: "2.0.0", To: "2.1.0", Latest: "2.1.0", Path: moved}) {
		t.Fatalf("updated = %+v", updated)
	}

	if got := contentOf(t, moved); got != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("%s holds %q, want the binary of 2.1.0", moved, got)
	}

	if info, _ := os.Stat(moved); info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}

	noLeftovers(t, moved)

	if want := [][]string{{"ln", "-sfn", moved, target}}; fmt.Sprint(sudo.asked) != fmt.Sprint(want) {
		t.Fatalf("sudo was asked %q, want %q", sudo.asked, want)
	}

	want := target + " is in " + filepath.Dir(target) + ", which only root can write: sudo is needed this " +
		"once, to move hm to " + filepath.Dir(moved) + " and leave a link to it in its place\n" +
		"hm now lives in " + moved + " and " + target + " links to it, so later updates need no sudo\n"
	if notice.String() != want {
		t.Fatalf("notice =\n%q\nwant\n%q", notice.String(), want)
	}
}

func TestSwitchMovesHmHomeTheSameWay(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.publish("2.0.0", false)
	engine, target := installed(t, source)
	rootOwned(t, target)
	sudo, home, _ := atTerminal(t, engine)
	moved := filepath.Join(home, ".hm", "bin", "hm")

	switched, err := engine.Switch(core.SwitchOptions{Running: "2.1.0", Version: "2.0.0"})
	if err != nil || switched.Path != moved || contentOf(t, moved) != "#!/bin/sh\n# 2.0.0\n" || len(sudo.asked) != 1 {
		t.Fatalf("switched = %+v, err = %v, sudo asked %v", switched, err, sudo.asked)
	}
}

func TestWithoutATerminalADirectoryOnlyRootWritesIsRefusedAsBefore(t *testing.T) {
	cases := map[string]func(*Engine){
		"no terminal": func(engine *Engine) { engine.install.interactive = func() bool { return false } },
		"no sudo":     func(engine *Engine) { engine.install.sudo = nil },
	}

	for name, without := range cases {
		t.Run(name, func(t *testing.T) {
			source := &fakeReleases{}
			source.publish("2.1.0", false)
			engine, target := installed(t, source)
			rootOwned(t, target)
			sudo, home, notice := atTerminal(t, engine)
			without(engine)

			_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

			refusal := refusalOf(t, err)
			if refusal.Kind != "install_dir_not_writable" || refusal.Code != 1 ||
				refusal.Message != "Cannot replace "+target+": "+filepath.Dir(target)+" is not writable by you, so 2.1.0 was not installed" ||
				!strings.Contains(refusal.Hint, installerOneLiner) || !strings.Contains(refusal.Hint, "HM_INSTALL_DIR") {
				t.Fatalf("refusal = %+v", refusal)
			}

			if len(sudo.asked) != 0 || notice.Len() != 0 || source.downloaded() != 0 || contentOf(t, target) != "old" {
				t.Fatalf("sudo asked %v, notice %q, downloaded %v", sudo.asked, notice.String(), source.asked)
			}

			if _, err := os.Stat(filepath.Join(home, ".hm")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("~/.hm was written: %v", err)
			}
		})
	}
}

// TestAnUnattendedReplacementNeverAsksSudo: a JSON answer is read by a program, with nobody to give
// sudo a password, so even with a person's terminal and sudo there the move to ~/.hm/bin is not
// tried, for update and for switch alike. The option says so; no variable of the process is read.
func TestAnUnattendedReplacementNeverAsksSudo(t *testing.T) {
	cases := map[string]func(*Engine) error{
		"update": func(engine *Engine) error {
			_, err := engine.Update(core.UpdateOptions{Running: "2.0.0", Unattended: true})
			return err
		},
		"switch": func(engine *Engine) error {
			_, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "2.1.0", Unattended: true})
			return err
		},
	}

	for name, replace := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HM_NON_INTERACTIVE", "")
			source := &fakeReleases{}
			source.publish("2.1.0", false)
			engine, target := installed(t, source)
			rootOwned(t, target)
			sudo, home, notice := atTerminal(t, engine)

			refusal := refusalOf(t, replace(engine))
			if refusal.Kind != "install_dir_not_writable" ||
				refusal.Message != "Cannot replace "+target+": "+filepath.Dir(target)+" is not writable by you, so 2.1.0 was not installed" {
				t.Fatalf("refusal = %+v", refusal)
			}

			if len(sudo.asked) != 0 || notice.Len() != 0 || contentOf(t, target) != "old" {
				t.Fatalf("sudo asked %v, notice %q", sudo.asked, notice.String())
			}

			if _, err := os.Stat(filepath.Join(home, ".hm")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("~/.hm was written: %v", err)
			}
		})
	}
}

func TestSudoRefusedLeavesTheOldBinaryRunning(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	rootOwned(t, target)
	sudo, home, _ := atTerminal(t, engine)
	sudo.err = errors.New("exit status 1")
	moved := filepath.Join(home, ".hm", "bin", "hm")

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "install_dir_not_writable" || refusal.Code != 1 ||
		refusal.Message != "Cannot replace "+target+": "+filepath.Dir(target)+" is not writable by you and sudo "+
			"did not link it (exit status 1), so 2.1.0 was not installed; "+target+" still runs as before, and "+
			moved+" is a copy nothing uses yet" ||
		!strings.Contains(refusal.Hint, installerOneLiner) || !strings.Contains(refusal.Hint, "HM_INSTALL_DIR") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" {
		t.Fatal("the old binary was replaced")
	}

	noLeftovers(t, target)
}

// homeBinary puts content at ~/.hm/bin/hm of home, executable, and returns its path.
func homeBinary(t *testing.T, home, content string) string {
	t.Helper()

	moved := filepath.Join(home, ".hm", "bin", "hm")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(moved, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	return moved
}

// earlierHm is an hm 2 binary as far as anyone asking its version can tell.
const earlierHm = "#!/bin/sh\necho 'hm 2.0.0-rc.2'\necho '  commit abc'\n"

func TestAFileAtTheHomeBinaryThatIsNotHmIsNeverOverwritten(t *testing.T) {
	cases := map[string]func(t *testing.T, home string) string{
		"a program that is not hm": func(t *testing.T, home string) string {
			return homeBinary(t, home, "#!/bin/sh\necho something else\n")
		},
		"the shell tool, hm 1.x": func(t *testing.T, home string) string {
			return homeBinary(t, home, "#!/bin/sh\necho 'hm 1.6.0'\n")
		},
		"a link": func(t *testing.T, home string) string {
			moved := homeBinary(t, home, "")
			elsewhere := filepath.Join(t.TempDir(), "hm")
			if err := os.WriteFile(elsewhere, []byte(earlierHm), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(elsewhere, moved); err != nil {
				t.Fatal(err)
			}

			return moved
		},
	}

	for name, existing := range cases {
		t.Run(name, func(t *testing.T) {
			source := &fakeReleases{}
			source.publish("2.1.0", false)
			engine, target := installed(t, source)
			rootOwned(t, target)
			sudo, home, notice := atTerminal(t, engine)
			moved := existing(t, home)
			before, _ := os.Readlink(moved)
			if before == "" {
				before = contentOf(t, moved)
			}

			_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

			refusal := refusalOf(t, err)
			if refusal.Kind != "install_dir_not_writable" || refusal.Code != 1 ||
				refusal.Message != "Cannot replace "+target+": "+filepath.Dir(target)+" is not writable by you, and "+
					moved+", where hm would move to, is not an hm, so 2.1.0 was not installed; neither was changed" ||
				!strings.Contains(refusal.Hint, "Move "+moved+" out of the way") ||
				!strings.Contains(refusal.Hint, "HM_INSTALL_DIR") {
				t.Fatalf("refusal = %+v", refusal)
			}

			after, _ := os.Readlink(moved)
			if after == "" {
				after = contentOf(t, moved)
			}
			if after != before || contentOf(t, target) != "old" {
				t.Fatalf("%s now holds %q (was %q), the old binary %q", moved, after, before, contentOf(t, target))
			}

			if len(sudo.asked) != 0 || notice.Len() != 0 || source.downloaded() != 0 {
				t.Fatalf("sudo asked %v, notice %q, downloaded %v", sudo.asked, notice.String(), source.asked)
			}

			noLeftovers(t, moved)
		})
	}
}

func TestAnEarlierHmAtTheHomeBinaryIsReplacedWhenSudoLinksIt(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	rootOwned(t, target)
	sudo, home, _ := atTerminal(t, engine)
	moved := homeBinary(t, home, earlierHm)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})
	if err != nil || updated.Path != moved || len(sudo.asked) != 1 {
		t.Fatalf("updated = %+v, err = %v, sudo asked %v", updated, err, sudo.asked)
	}

	if got := contentOf(t, moved); got != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("%s holds %q, want the binary of 2.1.0", moved, got)
	}

	noLeftovers(t, moved)
}

func TestSudoRefusedPutsBackTheHmThatWasAtTheHomeBinary(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target := installed(t, source)
	rootOwned(t, target)
	sudo, home, _ := atTerminal(t, engine)
	sudo.err = errors.New("exit status 1")
	moved := homeBinary(t, home, earlierHm)

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "install_dir_not_writable" || refusal.Code != 1 ||
		refusal.Message != "Cannot replace "+target+": "+filepath.Dir(target)+" is not writable by you and sudo "+
			"did not link it (exit status 1), so 2.1.0 was not installed; "+target+" still runs as before, and "+
			moved+" was put back as it was" ||
		!strings.Contains(refusal.Hint, installerOneLiner) {
		t.Fatalf("refusal = %+v", refusal)
	}

	if got := contentOf(t, moved); got != earlierHm {
		t.Fatalf("%s holds %q, want the hm that was there", moved, got)
	}

	if info, _ := os.Stat(moved); info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}

	if contentOf(t, target) != "old" {
		t.Fatal("the old binary was replaced")
	}

	noLeftovers(t, moved)
}

func TestTheRealInstallerAsksOnlyAPersonAtATerminal(t *testing.T) {
	original := inputIsTerminal
	t.Cleanup(func() { inputIsTerminal = original })

	cases := []struct {
		name           string
		terminal       bool
		nonInteractive string
		want           bool
	}{
		{"a terminal", true, "", true},
		{"HM_NON_INTERACTIVE", true, "1", false},
		{"a pipe", false, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inputIsTerminal = func() bool { return c.terminal }
			t.Setenv("HM_NON_INTERACTIVE", c.nonInteractive)

			if got := realInstaller().interactive(); got != c.want {
				t.Fatalf("interactive = %v, want %v", got, c.want)
			}
		})
	}
}

func TestABinaryBuiltFromACheckoutIsNotUpdated(t *testing.T) {
	for _, running := range []string{"", "dev", "1.6.0"} {
		source := &fakeReleases{}
		source.publish("2.1.0", false)
		engine, target := installed(t, source)

		_, err := engine.Update(core.UpdateOptions{Running: running})

		refusal := refusalOf(t, err)
		if refusal.Kind != "built_from_source" || refusal.Code != 6 ||
			!strings.Contains(refusal.Message, "built from a checkout") ||
			refusal.Hint != "Update it where it was built: git pull && go build -o bin/hm ./cmd/hm" {
			t.Fatalf("running %q: refusal = %+v", running, refusal)
		}

		if contentOf(t, target) != "old" || len(source.asked) != 0 {
			t.Fatalf("running %q: GitHub was asked %v", running, source.asked)
		}
	}
}

func TestReleasesThatCannotBeReadAreSaid(t *testing.T) {
	source := &fakeReleases{err: errors.New("api.github.com answered 503 Service Unavailable")}
	engine, target := installed(t, source)

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "releases_unavailable" || refusal.Code != 1 ||
		refusal.Message != "Could not read the releases of Hiberus Dockergento: api.github.com answered 503 Service Unavailable" ||
		!strings.Contains(refusal.Hint, "GITHUB_TOKEN") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" {
		t.Fatal("the executable was touched")
	}
}

// ------------------------------------------------------------------ switch

func TestSwitchInstallsTheReleaseAskedFor(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.publish("2.0.0", false)
	engine, target := installed(t, source)

	switched, err := engine.Switch(core.SwitchOptions{Running: "2.1.0", Version: "2.0.0"})
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if switched != (core.Switched{Reference: "2.0.0", Version: "2.0.0", From: "2.1.0", Path: target}) {
		t.Fatalf("switched = %+v", switched)
	}

	if contentOf(t, target) != "#!/bin/sh\n# 2.0.0\n" {
		t.Fatal("the executable is not the binary of 2.0.0")
	}

	noLeftovers(t, target)
}

func TestSwitchInstallsAPreReleaseByName(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0-rc.1", true)
	engine, target := installed(t, source)

	if _, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "v2.1.0-rc.1"}); err != nil ||
		contentOf(t, target) != "#!/bin/sh\n# 2.1.0-rc.1\n" {
		t.Fatalf("err = %v", err)
	}
}

func TestSwitchStableTakesTheNewestFinalRelease(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.2.0-rc.1", true)
	source.publish("2.0.0", false)
	source.publish("2.1.0", false)
	engine, target := installed(t, source)

	switched, err := engine.Switch(core.SwitchOptions{Running: "2.2.0-rc.1", Stable: true})
	if err != nil || switched.Version != "2.1.0" || contentOf(t, target) != "#!/bin/sh\n# 2.1.0\n" {
		t.Fatalf("switched = %+v, err = %v", switched, err)
	}
}

func TestSwitchStableWithNoFinalReleaseIsRefused(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0-rc.1", true)
	engine, target := installed(t, source)

	_, err := engine.Switch(core.SwitchOptions{Running: "2.0.0-rc.1", Stable: true})

	if refusal := refusalOf(t, err); refusal.Kind != "no_release" || refusal.Code != 2 || contentOf(t, target) != "old" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestSwitchingToTheRunningVersionChangesNothing(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0", false)
	engine, target := installed(t, source)

	switched, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "2.0.0"})
	if err != nil || !switched.Unchanged || contentOf(t, target) != "old" || source.downloaded() != 0 {
		t.Fatalf("switched = %+v, err = %v, asked %v", switched, err, source.asked)
	}
}

func TestAVersionOfTheShellToolIsRefusedWithoutAskingGitHub(t *testing.T) {
	source := &fakeReleases{}
	source.publish("1.5.1", false)
	engine, target := installed(t, source)

	_, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "1.5.1"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "version_before_go" || refusal.Code != 2 ||
		refusal.Message != "'1.5.1' is a version of the shell tool (1.x), which hm switch cannot install: nothing was changed" ||
		refusal.Hint != "Install it separately, in a directory of its own: git clone -b 1.5.1 "+
			"https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm-1.x, then run ~/hm-1.x/bin/run" {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" || len(source.asked) != 0 {
		t.Fatalf("GitHub was asked %v", source.asked)
	}
}

func TestABranchIsRefusedWithHowToBuildIt(t *testing.T) {
	source := &fakeReleases{}
	engine, target := installed(t, source)

	_, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "release/2.1.0"})

	refusal := refusalOf(t, err)
	if refusal.Kind != "not_a_version" || refusal.Code != 2 ||
		!strings.Contains(refusal.Message, "branches can no longer be switched to") ||
		!strings.Contains(refusal.Hint, "git clone -b release/2.1.0") || !strings.Contains(refusal.Hint, "go build") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" || len(source.asked) != 0 {
		t.Fatalf("GitHub was asked %v", source.asked)
	}
}

func TestAnUnknownVersionIsRefused(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0", false)
	engine, target := installed(t, source)

	_, err := engine.Switch(core.SwitchOptions{Running: "2.0.0", Version: "9.9.9"})

	if refusal := refusalOf(t, err); refusal.Kind != "unknown_version" || refusal.Code != 2 ||
		refusal.Message != "There is no release called '9.9.9'" || refusal.Hint != "hm switch --list" ||
		contentOf(t, target) != "old" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestADevelBuildIsNotSwitched(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0", false)
	engine, target := installed(t, source)

	_, err := engine.Switch(core.SwitchOptions{Running: "", Version: "2.0.0"})

	if refusal := refusalOf(t, err); refusal.Kind != "built_from_source" || contentOf(t, target) != "old" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestVersionsListsTheReleasesOfTheGoTool(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0-rc.9", true)
	source.publish("1.6.0", false)
	source.publish("2.0.0", false)
	source.publish("2.0.0-rc.10", true)
	source.publish("2.1.0", false)
	source.listed = append(source.listed, release{Tag: "2.3.0", Draft: true}, release{Tag: "nightly"})
	engine, _ := installed(t, source)

	versions, err := engine.Versions("2.0.0")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}

	want := core.Versions{Current: "2.0.0", Versions: []core.Version{
		{Name: "2.1.0"},
		{Name: "2.0.0", Installed: true},
		{Name: "2.0.0-rc.10", PreRelease: true},
		{Name: "2.0.0-rc.9", PreRelease: true},
	}}

	if fmt.Sprint(versions) != fmt.Sprint(want) {
		t.Fatalf("versions = %+v\nwant       %+v", versions, want)
	}
}

func TestVersionsOfADevelBuild(t *testing.T) {
	engine, _ := installed(t, &fakeReleases{})

	versions, err := engine.Versions("")
	if err != nil || versions.Current != "devel" || len(versions.Versions) != 0 {
		t.Fatalf("versions = %+v, err = %v", versions, err)
	}
}

func TestVersionsWhenGitHubCannotBeReadIsSaid(t *testing.T) {
	engine, _ := installed(t, &fakeReleases{err: errors.New("offline")})

	_, err := engine.Versions("2.0.0")

	if refusal := refusalOf(t, err); refusal.Kind != "releases_unavailable" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

// ------------------------------------------------------------------ the releases adapter

func TestReleasesAreReadFromAFileFixture(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "releases.json")
	asset := filepath.Join(dir, "hm_linux_amd64")

	if err := os.WriteFile(asset, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	listing := `[{"tag_name":"2.1.0","prerelease":false,"draft":false,` +
		`"assets":[{"name":"hm_linux_amd64","browser_download_url":"file://` + asset + `"}]}]`
	if err := os.WriteFile(fixture, []byte(listing), 0o644); err != nil {
		t.Fatal(err)
	}

	source := githubReleases{address: "file://" + fixture}

	releases, err := source.Releases()
	if err != nil || len(releases) != 1 || releases[0].Tag != "2.1.0" || releases[0].Assets["hm_linux_amd64"] != "file://"+asset {
		t.Fatalf("releases = %+v, err = %v", releases, err)
	}

	if body, err := source.Download(releases[0].Assets["hm_linux_amd64"], nil); err != nil || string(body) != "binary" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestTheReleasesVariableNamesTheFixture(t *testing.T) {
	t.Setenv(ReleasesURLVariable, "file:///somewhere/releases.json")

	if source := realInstaller().source.(githubReleases); source.address != "file:///somewhere/releases.json" {
		t.Fatalf("address = %q", source.address)
	}

	t.Setenv(ReleasesURLVariable, "")

	if source := realInstaller().source.(githubReleases); source.address != releasesAPI {
		t.Fatalf("address = %q, want GitHub's", source.address)
	}
}

// The token is GitHub's: a server that is not GitHub never sees it.
func TestTheTokenGoesToGitHubOnly(t *testing.T) {
	seen := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization") + "|" + r.URL.RawQuery
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	if _, err := (githubReleases{address: server.URL, token: "secret"}).Releases(); err != nil {
		t.Fatalf("Releases: %v", err)
	}

	if seen != "|per_page=100" {
		t.Fatalf("the server saw %q, want no token and one page of 100", seen)
	}
}

func TestARateLimitSaysHowToRaiseIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := (githubReleases{address: server.URL}).Releases()
	if err == nil || !strings.Contains(err.Error(), "rate limit") || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("err = %v", err)
	}
}

func TestVersionsCompareAsSemanticVersions(t *testing.T) {
	ordered := []string{"2.0.0-alpha", "2.0.0-rc.1", "2.0.0-rc.2", "2.0.0-rc.10", "2.0.0", "2.0.1", "2.1.0", "10.0.0"}

	for i := 0; i+1 < len(ordered); i++ {
		a, _ := parseVersion(ordered[i])
		b, _ := parseVersion(ordered[i+1])

		if !a.less(b) || b.less(a) {
			t.Fatalf("%s should come before %s", ordered[i], ordered[i+1])
		}
	}

	for _, text := range []string{"", "devel", "2.0", "2.0.0-", "main", "release/2.0.0"} {
		if _, ok := parseVersion(text); ok {
			t.Errorf("%q read as a version", text)
		}
	}
}

// A binary stamped with what `git describe --tags` says — a tag plus the commits past it — or with
// a GoReleaser snapshot or a dirty-tree stamp is a build past (or beside) that tag, not a
// pre-release before it. Read as semver it would sort below the tag, and update would "upgrade"
// it to the older release: it is a source build, and is refused like one.
func TestABinaryStampedWithGitDescribeIsNotDowngraded(t *testing.T) {
	for _, running := range []string{"2.0.0-5-gdeadbee", "2.0.0-12-g0123456789ab-dirty",
		"2.0.0-dirty", "2.0.1-SNAPSHOT-deadbee", "2.0.1-snapshot"} {
		source := &fakeReleases{}
		source.publish("2.0.0", false)
		engine, target := installed(t, source)

		_, err := engine.Update(core.UpdateOptions{Running: running})
		if refusal := refusalOf(t, err); refusal.Kind != "built_from_source" ||
			!strings.Contains(refusal.Message, "version "+running) {
			t.Fatalf("update from %q: refusal = %+v", running, refusal)
		}

		_, err = engine.Switch(core.SwitchOptions{Running: running, Stable: true})
		if refusal := refusalOf(t, err); refusal.Kind != "built_from_source" {
			t.Fatalf("switch from %q: refusal = %+v", running, refusal)
		}

		if contentOf(t, target) != "old" || len(source.asked) != 0 {
			t.Fatalf("running %q: the executable was touched or GitHub asked %v", running, source.asked)
		}
	}
}

// A real pre-release stamp is still a release: rc.1 updates to the final release.
func TestAPreReleaseStampIsStillARelease(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0", false)
	engine, target := installed(t, source)

	updated, err := engine.Update(core.UpdateOptions{Running: "2.0.0-rc.1"})
	if err != nil || !updated.Updated || contentOf(t, target) == "old" {
		t.Fatalf("updated = %+v, err = %v", updated, err)
	}
}

// ------------------------------------------------------------------ what a move shows on its way

// shown records what a move to another release shows, in order: its steps, the download, and the
// notices of the move to ~/.hm/bin, which it receives as their writer.
type shown struct{ events []string }

func (s *shown) watch() ports.ReleaseWatch {
	return ports.ReleaseWatch{
		Step: func(label string) func(string, bool) {
			s.events = append(s.events, "… "+label)

			return func(done string, failed bool) {
				switch {
				case failed:
					s.events = append(s.events, "✗ "+label)
				case done == "":
					s.events = append(s.events, "(nothing left)")
				default:
					s.events = append(s.events, "✓ "+done)
				}
			}
		},
		Download: func(total int64) (ports.CopyMeter, func(bool)) {
			s.events = append(s.events, fmt.Sprintf("download of %d bytes", total))

			carried := int64(0)

			return func(bytes, _ int64) { carried = bytes }, func(failed bool) {
				s.events = append(s.events, fmt.Sprintf("downloaded %d bytes, failed %v", carried, failed))
			}
		},
	}
}

func (s *shown) Write(text []byte) (int, error) {
	s.events = append(s.events, "notice: "+strings.TrimSpace(string(text)))

	return len(text), nil
}

// watched is installed with something watching what the move shows.
func watched(t *testing.T, source *fakeReleases) (*Engine, string, *shown) {
	t.Helper()

	engine, target := installed(t, source)
	seen := &shown{}
	engine.options.Releasing = seen.watch()

	return engine, target, seen
}

func sameEvents(t *testing.T, got, want []string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("shown:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// downloadOf is the two events of downloading the fake binary of tag, which is that many bytes.
func downloadOf(tag string) []string {
	size := len("#!/bin/sh\n# " + tag + "\n")

	return []string{
		fmt.Sprintf("download of %d bytes", size),
		fmt.Sprintf("downloaded %d bytes, failed false", size),
	}
}

func TestAnUpdateShowsEachStepAsItHappens(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	source.publish("2.0.0", false)
	engine, target, seen := watched(t, source)

	if _, err := engine.Update(core.UpdateOptions{Running: "2.0.0"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	want := append([]string{"… Looking up the latest release", "✓ Latest release: 2.1.0 (you have 2.0.0)"},
		downloadOf("2.1.0")...)
	want = append(want, "… Verifying the checksum", "✓ Checksum verified",
		"… Installing 2.1.0", "✓ Installed at "+target)
	sameEvents(t, seen.events, want)
}

func TestAnUpdateToAPreReleaseSaysItIsOne(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.0.0-rc.5", true)
	source.publish("2.0.0-rc.4", true)
	engine, _, seen := watched(t, source)

	if _, err := engine.Update(core.UpdateOptions{Running: "2.0.0-rc.4", PreReleases: true}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(seen.events) < 2 || seen.events[1] != "✓ Latest pre-release: 2.0.0-rc.5 (you have 2.0.0-rc.4)" {
		t.Fatalf("shown %q", seen.events)
	}
}

// Up to date, the answer is the one the command prints: the step leaves nothing of its own.
func TestAnUpToDateUpdateShowsOnlyTheLookUp(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, _, seen := watched(t, source)

	if _, err := engine.Update(core.UpdateOptions{Running: "2.1.0"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	sameEvents(t, seen.events, []string{"… Looking up the latest release", "(nothing left)"})
}

// A step that fails is shown failed, and the error that follows is the one it always was.
func TestAFailedStepIsShownFailed(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(source *fakeReleases)
		want    []string
		kind    string
	}{
		{
			name:    "the releases cannot be read",
			prepare: func(source *fakeReleases) { source.err = errors.New("offline") },
			want:    []string{"… Looking up the latest release", "✗ Looking up the latest release"},
			kind:    "releases_unavailable",
		},
		{
			name: "the binary cannot be downloaded",
			prepare: func(source *fakeReleases) {
				delete(source.files, "https://example.invalid/2.1.0/hm_linux_amd64")
			},
			want: []string{"… Looking up the latest release", "✓ Latest release: 2.1.0 (you have 2.0.0)",
				"… Downloading hm_linux_amd64", "✗ Downloading hm_linux_amd64"},
			kind: "download_failed",
		},
		{
			name: "the checksum does not match",
			prepare: func(source *fakeReleases) {
				source.files["https://example.invalid/2.1.0/hm_linux_amd64"] = "tampered"
			},
			want: []string{"… Looking up the latest release", "✓ Latest release: 2.1.0 (you have 2.0.0)",
				"download of 8 bytes", "downloaded 8 bytes, failed false",
				"… Verifying the checksum", "✗ Verifying the checksum"},
			kind: "download_failed",
		},
		{
			name:    "the release publishes no checksums",
			prepare: func(source *fakeReleases) { delete(source.listed[0].Assets, "checksums.txt") },
			want: []string{"… Looking up the latest release", "✓ Latest release: 2.1.0 (you have 2.0.0)",
				"… Verifying the checksum", "✗ Verifying the checksum"},
			kind: "download_failed",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := &fakeReleases{}
			source.publish("2.1.0", false)
			c.prepare(source)
			engine, target, seen := watched(t, source)

			_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

			if refusal := refusalOf(t, err); refusal.Kind != c.kind {
				t.Fatalf("refusal = %+v, want %s", refusal, c.kind)
			}

			sameEvents(t, seen.events, c.want)

			if contentOf(t, target) != "old" {
				t.Fatal("the old binary was replaced")
			}
		})
	}
}

func TestAnInstallThatCannotRenameIsShownFailed(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target, seen := watched(t, source)

	// A directory where the executable was: the rename over it fails after the download
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := engine.Update(core.UpdateOptions{Running: "2.0.0"})

	if refusal := refusalOf(t, err); refusal.Kind != "install_failed" {
		t.Fatalf("refusal = %+v", refusal)
	}

	if last := seen.events[len(seen.events)-2:]; last[0] != "… Installing 2.1.0" || last[1] != "✗ Installing 2.1.0" {
		t.Fatalf("shown %q, want the install shown failed", seen.events)
	}
}

// The move to ~/.hm/bin says what it does on lines of its own, with no step running while sudo may
// ask for the password.
func TestTheMoveHomeNoticesStandBetweenTheSteps(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target, seen := watched(t, source)
	rootOwned(t, target)
	_, home, _ := atTerminal(t, engine)
	engine.install.notice = seen
	moved := filepath.Join(home, ".hm", "bin", "hm")

	if _, err := engine.Update(core.UpdateOptions{Running: "2.0.0"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	want := append([]string{"… Looking up the latest release", "✓ Latest release: 2.1.0 (you have 2.0.0)"},
		downloadOf("2.1.0")...)
	want = append(want, "… Verifying the checksum", "✓ Checksum verified",
		"notice: "+target+" is in "+filepath.Dir(target)+", which only root can write: sudo is needed this "+
			"once, to move hm to "+filepath.Dir(moved)+" and leave a link to it in its place",
		"notice: hm now lives in "+moved+" and "+target+" links to it, so later updates need no sudo",
		"… Installing 2.1.0", "✓ Installed at "+moved)
	sameEvents(t, seen.events, want)
}

func TestAMoveHomeSudoRefusedIsShownFailed(t *testing.T) {
	source := &fakeReleases{}
	source.publish("2.1.0", false)
	engine, target, seen := watched(t, source)
	rootOwned(t, target)
	sudo, _, _ := atTerminal(t, engine)
	sudo.err = errors.New("no password")
	engine.install.notice = seen

	if _, err := engine.Update(core.UpdateOptions{Running: "2.0.0"}); err == nil {
		t.Fatal("Update succeeded without sudo")
	}

	if last := seen.events[len(seen.events)-2:]; last[0] != "… Installing 2.1.0" || last[1] != "✗ Installing 2.1.0" {
		t.Fatalf("shown %q, want the install shown failed after the notice", seen.events)
	}
}

func TestASwitchShowsTheReleaseAskedFor(t *testing.T) {
	cases := []struct {
		name    string
		options core.SwitchOptions
		want    []string
	}{
		{
			name:    "a version",
			options: core.SwitchOptions{Running: "2.1.0", Version: "2.0.0"},
			want:    []string{"… Looking up release 2.0.0", "✓ Release: 2.0.0 (you have 2.1.0)"},
		},
		{
			name:    "the newest final release",
			options: core.SwitchOptions{Running: "2.0.0", Stable: true},
			want:    []string{"… Looking up the latest final release", "✓ Latest release: 2.1.0 (you have 2.0.0)"},
		},
		{
			name:    "the one running",
			options: core.SwitchOptions{Running: "2.1.0", Version: "2.1.0"},
			want:    []string{"… Looking up release 2.1.0", "(nothing left)"},
		},
		{
			name:    "a release that does not exist",
			options: core.SwitchOptions{Running: "2.1.0", Version: "2.9.0"},
			want:    []string{"… Looking up release 2.9.0", "✗ Looking up release 2.9.0"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := &fakeReleases{}
			source.publish("2.1.0", false)
			source.publish("2.0.0", false)
			engine, _, seen := watched(t, source)

			_, _ = engine.Switch(c.options)

			if len(seen.events) < 2 {
				t.Fatalf("shown %q", seen.events)
			}

			sameEvents(t, seen.events[:2], c.want)
		})
	}
}

// What is refused before anything is read shows nothing: there was no step.
func TestASwitchRefusedBeforeLookingShowsNothing(t *testing.T) {
	engine, _, seen := watched(t, &fakeReleases{})

	if _, err := engine.Switch(core.SwitchOptions{Running: "2.1.0", Version: "main"}); err == nil {
		t.Fatal("a branch was accepted")
	}

	if len(seen.events) != 0 {
		t.Fatalf("shown %q, want nothing", seen.events)
	}
}

// ------------------------------------------------------------------ the download's progress

// downloadSeen records what a download showed: the total announced, the bytes reached and how it
// ended.
type downloadSeen struct {
	started bool
	total   int64
	bytes   int64
	ended   int
	failed  bool
}

func (d *downloadSeen) watch(total int64) (ports.CopyMeter, func(bool)) {
	d.started, d.total = true, total

	return func(bytes, _ int64) { d.bytes = bytes }, func(failed bool) { d.ended++; d.failed = failed }
}

func TestADownloadsTotalIsTheContentLength(t *testing.T) {
	body := strings.Repeat("b", 64<<10)

	cases := []struct {
		name      string
		handler   http.HandlerFunc
		wantTotal int64
	}{
		{"announced", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			_, _ = w.Write([]byte(body))
		}, int64(len(body))},
		{"not announced", func(w http.ResponseWriter, _ *http.Request) {
			// Flushed before the body is whole, so it goes chunked, with no length
			_, _ = w.Write([]byte(body[:10]))
			w.(http.Flusher).Flush()
			_, _ = w.Write([]byte(body[10:]))
		}, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(c.handler)
			defer server.Close()

			seen := &downloadSeen{}

			got, err := (githubReleases{}).Download(server.URL+"/hm_linux_amd64", seen.watch)
			if err != nil || string(got) != body {
				t.Fatalf("Download: %d bytes, err = %v", len(got), err)
			}

			if seen.total != c.wantTotal || seen.bytes != int64(len(body)) || seen.ended != 1 || seen.failed {
				t.Fatalf("shown %+v, want total %d, every byte and one good end", seen, c.wantTotal)
			}
		})
	}
}

func TestADownloadCutShortEndsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("only this"))
	}))
	defer server.Close()

	seen := &downloadSeen{}

	if _, err := (githubReleases{}).Download(server.URL+"/hm_linux_amd64", seen.watch); err == nil {
		t.Fatal("a body cut short was taken whole")
	}

	if seen.total != 1000 || seen.ended != 1 || !seen.failed {
		t.Fatalf("shown %+v, want the total announced and one failed end", seen)
	}
}

// A server that refuses never starts the line: there is nothing being downloaded to show.
func TestADownloadRefusedShowsNoLine(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	seen := &downloadSeen{}

	if _, err := (githubReleases{}).Download(server.URL+"/hm_linux_amd64", seen.watch); err == nil {
		t.Fatal("a 404 was taken for a binary")
	}

	if seen.started {
		t.Fatalf("shown %+v, want nothing", seen)
	}
}

// The fixture HM_RELEASES_URL names is read from disk, and its size is the total.
func TestADownloadFromAFileFixtureKnowsItsSize(t *testing.T) {
	asset := filepath.Join(t.TempDir(), "hm_linux_amd64")
	if err := os.WriteFile(asset, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	seen := &downloadSeen{}

	if got, err := (githubReleases{}).Download("file://"+asset, seen.watch); err != nil || string(got) != "binary" {
		t.Fatalf("Download: %q, err = %v", got, err)
	}

	if seen.total != 6 || seen.bytes != 6 || seen.ended != 1 || seen.failed {
		t.Fatalf("shown %+v", seen)
	}
}
