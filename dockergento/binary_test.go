package dockergento

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//
// The compiled command follows the checkout. Never the network nor a toolchain here: every engine
// gets a fake source, and TestMain replaces the default one so that an engine built without it —
// the update and switch suites — gets a builder that writes a stub instead of reaching GitHub.
//

func TestMain(m *testing.M) {
	newBinarySource = func() binarySource {
		return binarySource{
			download: func(url string) ([]byte, error) { return nil, errors.New("no network in tests") },
			build: func(root, output, version string) error {
				return os.WriteFile(output, []byte("#!/bin/sh\n# built\n"), 0o644)
			},
			goos: "linux", goarch: "amd64",
		}
	}

	os.Exit(m.Run())
}

// fakeSource serves files by URL and records what it was asked; build is set per test.
type fakeSource struct {
	files     map[string][]byte
	requested []string
	built     []string
}

func (f *fakeSource) source(build func(root, output, version string) error) *binarySource {
	return &binarySource{
		download: func(url string) ([]byte, error) {
			f.requested = append(f.requested, url)
			if body, ok := f.files[url]; ok {
				return body, nil
			}
			return nil, fmt.Errorf("%s answered 404 Not Found", url)
		},
		build: build,
		goos:  "darwin", goarch: "arm64",
	}
}

func (f *fakeSource) builder(content string) func(root, output, version string) error {
	return func(root, output, version string) error {
		f.built = append(f.built, version)
		return os.WriteFile(output, []byte(content), 0o644)
	}
}

func checksumLine(content, name string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:]) + "  " + name + "\n"
}

const assetURL = releaseDownloads + "/2.0.0/hm_darwin_arm64"
const checksumsURL = releaseDownloads + "/2.0.0/checksums.txt"

// released is an installation whose HEAD is the tag 2.0.0, with an old binary in bin/.
func released(t *testing.T) string {
	t.Helper()

	_, installed := installation(t)
	git(t, installed, "tag", "2.0.0")
	git(t, installed, "checkout", "-q", "2.0.0")
	writeOldBinary(t, installed)

	return installed
}

func writeOldBinary(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "hm"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func binaryOf(t *testing.T, root string) (string, os.FileMode) {
	t.Helper()

	info, err := os.Stat(filepath.Join(root, "bin", "hm"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "bin", "hm"))
	if err != nil {
		t.Fatal(err)
	}

	return string(content), info.Mode().Perm()
}

// noLeftovers: an atomic replace leaves nothing beside bin/hm, whether it succeeded or not.
func noLeftovers(t *testing.T, root string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "hm" {
			t.Errorf("bin/ keeps %s after the replace", entry.Name())
		}
	}
}

func engineWith(root string, source *binarySource) *Engine {
	engine := New(Options{Root: root})
	engine.binaries = source

	return engine
}

func TestAReleaseCheckoutGetsItsVerifiedReleaseAsset(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{files: map[string][]byte{
		assetURL:     []byte("new binary"),
		checksumsURL: []byte(checksumLine("other", "hm_linux_amd64") + checksumLine("new binary", "hm_darwin_arm64")),
	}}

	if err := engineWith(installed, fake.source(fake.builder("built"))).RefreshBinary(); err != nil {
		t.Fatalf("RefreshBinary = %v, want no error", err)
	}

	if content, mode := binaryOf(t, installed); content != "new binary" || mode != 0o755 {
		t.Errorf("bin/hm = %q %v, want the release asset, executable", content, mode)
	}
	if len(fake.built) != 0 {
		t.Errorf("built %v although the release asset was there", fake.built)
	}
	if strings.Join(fake.requested, " ") != assetURL+" "+checksumsURL {
		t.Errorf("requested %v, want the tag's asset and checksums", fake.requested)
	}
	noLeftovers(t, installed)
}

func TestADownloadThatFailsIsBuiltInstead(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{}

	if err := engineWith(installed, fake.source(fake.builder("built"))).RefreshBinary(); err != nil {
		t.Fatalf("RefreshBinary = %v, want no error", err)
	}

	if content, mode := binaryOf(t, installed); content != "built" || mode != 0o755 {
		t.Errorf("bin/hm = %q %v, want the built one, executable", content, mode)
	}
	if len(fake.built) != 1 || fake.built[0] != "2.0.0" {
		t.Errorf("built with versions %v, want one build stamped 2.0.0", fake.built)
	}
}

func TestAChecksumMismatchIsNoBinary(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{files: map[string][]byte{
		assetURL:     []byte("tampered"),
		checksumsURL: []byte(checksumLine("new binary", "hm_darwin_arm64")),
	}}

	err := engineWith(installed, fake.source(nil)).RefreshBinary()

	refusal := refusalOf(t, err)
	if refusal.Kind != "binary_not_refreshed" || refusal.Code != 1 ||
		!strings.Contains(refusal.Message, "does not match its checksum") {
		t.Fatalf("refusal = %+v, want binary_not_refreshed naming the checksum", refusal)
	}
	if content, _ := binaryOf(t, installed); content != "old" {
		t.Errorf("bin/hm = %q, want the old binary kept", content)
	}
	noLeftovers(t, installed)
}

func TestAnAssetMissingFromTheChecksumsIsNoBinary(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{files: map[string][]byte{
		assetURL:     []byte("new binary"),
		checksumsURL: []byte(checksumLine("new binary", "hm_linux_amd64")),
	}}

	err := engineWith(installed, fake.source(fake.builder("built"))).RefreshBinary()
	if err != nil {
		t.Fatalf("RefreshBinary = %v, want the build to take over", err)
	}
	if content, _ := binaryOf(t, installed); content != "built" {
		t.Errorf("bin/hm = %q, want the build, not the unlisted asset", content)
	}
}

func TestWhenNothingWorksTheOldBinaryStays(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{}
	failing := func(root, output, version string) error {
		// Half a binary written before failing must not reach bin/hm
		_ = os.WriteFile(output, []byte("half"), 0o644)
		return errors.New("cannot find main module")
	}

	err := engineWith(installed, fake.source(failing)).RefreshBinary()

	refusal := refusalOf(t, err)
	if refusal.Kind != "binary_not_refreshed" || refusal.Code != 1 ||
		!strings.Contains(refusal.Message, "2.0.0") ||
		!strings.Contains(refusal.Message, "404") ||
		!strings.Contains(refusal.Message, "cannot find main module") ||
		!strings.Contains(refusal.Hint, "go build -o bin/hm ./cmd/hm") {
		t.Fatalf("refusal = %+v, want the version, both reasons and how to build", refusal)
	}
	if content, mode := binaryOf(t, installed); content != "old" || mode != 0o755 {
		t.Errorf("bin/hm = %q %v, want the old binary kept as it was", content, mode)
	}
	noLeftovers(t, installed)
}

func TestABranchIsOnlyEverBuilt(t *testing.T) {
	_, installed := installation(t)
	writeOldBinary(t, installed)
	fake := &fakeSource{}

	if err := engineWith(installed, fake.source(fake.builder("built"))).RefreshBinary(); err != nil {
		t.Fatalf("RefreshBinary = %v, want no error", err)
	}
	if len(fake.requested) != 0 {
		t.Errorf("downloaded %v for a checkout that is not a release", fake.requested)
	}
	if content, _ := binaryOf(t, installed); content != "built" {
		t.Errorf("bin/hm = %q, want the built one", content)
	}
}

func TestABranchWithoutAToolchainKeepsTheOldBinary(t *testing.T) {
	_, installed := installation(t)
	writeOldBinary(t, installed)
	fake := &fakeSource{}

	refusal := refusalOf(t, engineWith(installed, fake.source(nil)).RefreshBinary())
	if refusal.Kind != "binary_not_refreshed" || !strings.Contains(refusal.Message, "not a release") ||
		!strings.Contains(refusal.Message, "no Go toolchain") {
		t.Fatalf("refusal = %+v, want it to say why neither way worked", refusal)
	}
	if content, _ := binaryOf(t, installed); content != "old" {
		t.Errorf("bin/hm = %q, want the old binary kept", content)
	}
}

func TestAPlatformWithoutReleaseAssetsIsBuilt(t *testing.T) {
	installed := released(t)
	fake := &fakeSource{}
	source := fake.source(fake.builder("built"))
	source.goos = "windows"

	if err := engineWith(installed, source).RefreshBinary(); err != nil {
		t.Fatalf("RefreshBinary = %v, want no error", err)
	}
	if len(fake.requested) != 0 {
		t.Errorf("downloaded %v for a platform releases do not publish", fake.requested)
	}
}

// TestUpdateRefreshesTheBinaryAfterThePull: what was pulled is what the binary is obtained for.
func TestUpdateRefreshesTheBinaryAfterThePull(t *testing.T) {
	_, installed := installation(t)
	writeOldBinary(t, installed)
	fake := &fakeSource{}

	if err := engineWith(installed, fake.source(fake.builder("built"))).Update(); err != nil {
		t.Fatalf("Update = %v, want no error", err)
	}
	if content, _ := binaryOf(t, installed); content != "built" {
		t.Errorf("bin/hm = %q, want it refreshed by the update", content)
	}
}

// TestAnUpdateWhoseBinaryFailsKeepsThePullAndSaysSo: the pull is not undone, the completion is
// still regenerated, and the refusal is what the command exits with.
func TestAnUpdateWhoseBinaryFailsKeepsThePullAndSaysSo(t *testing.T) {
	origin, installed := installation(t)
	writeOldBinary(t, installed)
	if err := os.WriteFile(filepath.Join(origin, "NEW"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-qm", "second")
	fake := &fakeSource{}

	refusal := refusalOf(t, engineWith(installed, fake.source(nil)).Update())
	if refusal.Kind != "binary_not_refreshed" || refusal.Code != 1 {
		t.Fatalf("refusal = %+v, want binary_not_refreshed with code 1", refusal)
	}
	if _, err := os.Stat(filepath.Join(installed, "NEW")); err != nil {
		t.Errorf("the pull was undone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed, "completion-generated")); err != nil {
		t.Errorf("the completion was not regenerated: %v", err)
	}
	if content, _ := binaryOf(t, installed); content != "old" {
		t.Errorf("bin/hm = %q, want the old binary kept", content)
	}
}

// TestSwitchingToAReleaseGetsThatReleasesBinary: the tag switched to, not latest.
func TestSwitchingToAReleaseGetsThatReleasesBinary(t *testing.T) {
	_, installed := versioned(t)
	writeOldBinary(t, installed)
	url := releaseDownloads + "/1.10.0/hm_darwin_arm64"
	fake := &fakeSource{files: map[string][]byte{
		url: []byte("1.10 binary"),
		releaseDownloads + "/1.10.0/checksums.txt": []byte(checksumLine("1.10 binary", "hm_darwin_arm64")),
	}}

	if _, err := engineWith(installed, fake.source(nil)).Switch("1.10.0"); err != nil {
		t.Fatalf("Switch = %v, want no error", err)
	}
	if content, _ := binaryOf(t, installed); content != "1.10 binary" {
		t.Errorf("bin/hm = %q, want the 1.10.0 release asset", content)
	}
}

// TestASwitchWhoseBinaryFailsStaysSwitchedAndSaysSo: the checkout is not moved back.
func TestASwitchWhoseBinaryFailsStaysSwitchedAndSaysSo(t *testing.T) {
	_, installed := versioned(t)
	writeOldBinary(t, installed)
	fake := &fakeSource{}

	_, err := engineWith(installed, fake.source(nil)).Switch("1.2.0")

	refusal := refusalOf(t, err)
	if refusal.Kind != "binary_not_refreshed" || !strings.Contains(refusal.Message, "1.2.0") {
		t.Fatalf("refusal = %+v, want binary_not_refreshed naming 1.2.0", refusal)
	}
	if now := output(t, installed, "describe", "--tags"); now != "1.2.0" {
		t.Errorf("installation at %s, want it left switched to 1.2.0", now)
	}
	if content, _ := binaryOf(t, installed); content != "old" {
		t.Errorf("bin/hm = %q, want the old binary kept", content)
	}
}

// TestTheBinarySourceVariableIsInertUnlessSet: an ordinary environment downloads and builds.
func TestTheBinarySourceVariableIsInertUnlessSet(t *testing.T) {
	t.Setenv(BinarySourceVariable, "")

	if source := realBinarySource(); source.local != "" || source.download == nil {
		t.Fatalf("realBinarySource with %s empty = %+v, want the download and no local file", BinarySourceVariable, source)
	}
}

// TestTheBinarySourceVariableInstallsThatFileAndNothingElse: what suites running update and switch
// for real use, so they reach neither GitHub nor a toolchain.
func TestTheBinarySourceVariableInstallsThatFileAndNothingElse(t *testing.T) {
	installed := released(t)
	local := filepath.Join(t.TempDir(), "hm-under-test")
	if err := os.WriteFile(local, []byte("binary under test"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(BinarySourceVariable, local)

	source := realBinarySource()
	fake := &fakeSource{}
	recording := fake.source(fake.builder("built"))
	source.download, source.build = recording.download, recording.build

	if err := engineWith(installed, &source).RefreshBinary(); err != nil {
		t.Fatalf("RefreshBinary = %v, want no error", err)
	}
	if content, mode := binaryOf(t, installed); content != "binary under test" || mode != 0o755 {
		t.Errorf("bin/hm = %q %v, want the file %s names, executable", content, mode, BinarySourceVariable)
	}
	if len(fake.requested) != 0 || len(fake.built) != 0 {
		t.Errorf("downloaded %v and built %v with %s set", fake.requested, fake.built, BinarySourceVariable)
	}
	noLeftovers(t, installed)
}

// TestABinarySourceThatIsMissingKeepsTheOldBinary: set means nothing else is tried, and a file that
// is not there is said like any other failure.
func TestABinarySourceThatIsMissingKeepsTheOldBinary(t *testing.T) {
	installed := released(t)
	t.Setenv(BinarySourceVariable, filepath.Join(t.TempDir(), "missing"))

	source := realBinarySource()
	fake := &fakeSource{}
	recording := fake.source(fake.builder("built"))
	source.download, source.build = recording.download, recording.build

	refusal := refusalOf(t, engineWith(installed, &source).RefreshBinary())
	if refusal.Kind != "binary_not_refreshed" || !strings.Contains(refusal.Message, BinarySourceVariable) {
		t.Fatalf("refusal = %+v, want binary_not_refreshed naming %s", refusal, BinarySourceVariable)
	}
	if len(fake.requested) != 0 || len(fake.built) != 0 {
		t.Errorf("downloaded %v and built %v with %s set", fake.requested, fake.built, BinarySourceVariable)
	}
	if content, _ := binaryOf(t, installed); content != "old" {
		t.Errorf("bin/hm = %q, want the old binary kept", content)
	}
	noLeftovers(t, installed)
}
