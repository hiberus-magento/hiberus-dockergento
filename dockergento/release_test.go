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

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
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

func (f *fakeReleases) Download(url string) ([]byte, error) {
	f.asked = append(f.asked, url)

	content, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("%s answered 404 Not Found", url)
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

	dir := t.TempDir()
	target := filepath.Join(dir, "hm")

	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	engine := New(Options{Root: t.TempDir()})
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
		!strings.Contains(refusal.Hint, "sudo") || !strings.Contains(refusal.Hint, "HM_INSTALL_DIR") {
		t.Fatalf("refusal = %+v", refusal)
	}

	if contentOf(t, target) != "old" || source.downloaded() != 0 {
		t.Fatalf("the executable was touched or something downloaded: %v", source.asked)
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

	if body, err := source.Download(releases[0].Assets["hm_linux_amd64"]); err != nil || string(body) != "binary" {
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
