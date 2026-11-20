package dockergento

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The compiled command follows the checkout.
//
// The installation is a git checkout and bin/hm is not in it (.gitignore), so moving the checkout —
// `hm update`, `hm switch` — leaves the binary where it was: an old binary answering over new shell
// and data files. After every move the binary is obtained again for what is checked out now: the
// release asset when HEAD is exactly a release tag, verified against that release's checksums.txt,
// or built with a local Go toolchain. A ref that is not a release is only ever built: a release
// asset of another version would be the same disagreement again. When the process itself is bin/hm
// built from exactly HEAD there is nothing to obtain, so an update that pulled nothing does not need
// a toolchain.
//
// Without a binary for the new checkout the move is kept and the command fails: the binary is the
// program, so reporting success would claim an update of the program that did not happen.
//
// The replacement is atomic — a temporary file in bin/, made executable, renamed over bin/hm — and a
// failure leaves the previous binary in place. Renaming over the running executable is safe on
// Linux and macOS: the process keeps the inode it was started from, and nothing here executes the
// path again.
//

// releaseDownloads is where tagged releases publish their assets: hm_<os>_<arch> and checksums.txt.
const releaseDownloads = "https://github.com/hiberus-magento/hiberus-dockergento/releases/download"

// binarySource is how a binary is obtained. Injected so tests never reach the network or a
// toolchain.
type binarySource struct {
	// download returns the body of a URL, or an error for anything but a whole 200.
	download func(url string) ([]byte, error)

	// build compiles ./cmd/hm of root into output, stamping version. Nil when there is no Go
	// toolchain on this machine.
	build func(root, output, version string) error

	goos, goarch string

	// local, when set, is a file copied as the new binary instead of downloading or building one.
	local string
}

// BinarySourceVariable names a local file to install as bin/hm whenever the installation moves,
// instead of downloading a release asset or building one. It is for test suites that run `hm
// update` or `hm switch` for real and must reach neither the network nor a toolchain: they point it
// at a copy of the binary under test. Unset or empty, it does nothing; set, nothing else is tried,
// and a file that cannot be copied is a binary_not_refreshed like any other failure.
const BinarySourceVariable = "HM_BINARY_SOURCE"

// realBinarySource is HTTP and the go on PATH, or the file BinarySourceVariable names.
func realBinarySource() binarySource {
	source := binarySource{download: httpDownload, goos: runtime.GOOS, goarch: runtime.GOARCH}

	if local := os.Getenv(BinarySourceVariable); local != "" {
		source.local = local
		return source
	}

	if _, err := exec.LookPath("go"); err == nil {
		source.build = goBuild
	}

	return source
}

// newBinarySource is what an engine without an injected source uses. A variable so the package's
// tests can replace it for every engine they build.
var newBinarySource = realBinarySource

func httpDownload(url string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}

	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, response.Status)
	}

	// A binary is tens of megabytes; anything past this is not one
	return io.ReadAll(io.LimitReader(response.Body, 512<<20))
}

func goBuild(root, output, version string) error {
	args := []string{"build"}
	if version != "" {
		args = append(args, "-ldflags", "-X github.com/hiberus-magento/hiberus-dockergento/internal/cli.Version="+version)
	}
	args = append(args, "-o", output, "./cmd/hm")

	command := exec.Command("go", args...)
	command.Dir = root

	if said, err := command.CombinedOutput(); err != nil {
		if text := strings.TrimSpace(string(said)); text != "" {
			return errors.New(text)
		}

		return err
	}

	return nil
}

// runningBuild is the executable of this process, resolved, and the commit it was built from, ok
// only when the build says which commit and was not built from a modified tree. A variable so that
// tests do not depend on how the test binary was built.
var runningBuild = func() (path, revision string, ok bool) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", false
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	info, available := debug.ReadBuildInfo()
	if !available {
		return "", "", false
	}

	modified := true
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	return executable, revision, revision != "" && !modified
}

// alreadyCurrent: the process is bin/hm of root and was built from exactly HEAD, so bin/hm already
// matches the checkout and there is nothing to obtain.
func alreadyCurrent(root string, git installationGit) bool {
	path, revision, ok := runningBuild()
	if !ok {
		return false
	}

	target := filepath.Join(root, "bin", "hm")
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	head, err := git.run("rev-parse", "HEAD")

	return err == nil && path == target && head == revision
}

func (e *Engine) binarySource() binarySource {
	if e.binaries != nil {
		return *e.binaries
	}

	return newBinarySource()
}

// releaseAsset is the asset name of this platform, or "" when releases do not publish one for it.
func releaseAsset(goos, goarch string) string {
	if (goos != "darwin" && goos != "linux") || (goarch != "amd64" && goarch != "arm64") {
		return ""
	}

	return "hm_" + goos + "_" + goarch
}

// RefreshBinary obtains the compiled command for what the installation has checked out now and
// puts it at bin/hm. On failure the previous binary is untouched and the refusal says why.
func (e *Engine) RefreshBinary() error {
	root := e.options.Root
	git := installationGit(root)
	source := e.binarySource()
	target := filepath.Join(root, "bin", "hm")

	// HM_BINARY_SOURCE always installs: it is what a test suite asked for
	if source.local == "" && alreadyCurrent(root, git) {
		return nil
	}

	tag, err := git.run("describe", "--tags", "--exact-match", "HEAD")
	if err != nil {
		tag = ""
	}

	version, err := git.run("describe", "--tags")
	if err != nil {
		version = ""
	}

	reasons := []string{}

	if source.local != "" {
		err := replaceWith(target, func(path string) error { return copyFile(source.local, path) })
		if err == nil {
			return nil
		}
		reasons = append(reasons, fmt.Sprintf("%s=%s could not be installed: %v", BinarySourceVariable, source.local, err))
	} else if tag != "" {
		if asset := releaseAsset(source.goos, source.goarch); asset == "" {
			reasons = append(reasons, fmt.Sprintf("releases publish no binary for %s/%s", source.goos, source.goarch))
		} else if binary, err := releaseBinary(source, tag, asset); err != nil {
			reasons = append(reasons, err.Error())
		} else if err := replaceWith(target, func(path string) error { return os.WriteFile(path, binary, 0o755) }); err != nil {
			reasons = append(reasons, "could not install the downloaded binary: "+err.Error())
		} else {
			return nil
		}
	} else {
		reasons = append(reasons, "the checkout is not a release, so there is no binary to download for it")
	}

	switch {
	case source.local != "":
		// Set means nothing else is tried
	case source.build == nil:
		reasons = append(reasons, "there is no Go toolchain to build it")
	default:
		err := replaceWith(target, func(path string) error { return source.build(root, path, version) })
		if err == nil {
			return nil
		}
		reasons = append(reasons, "building it failed: "+err.Error())
	}

	now := version
	if now == "" {
		now = git.current()
	}

	return core.Refusal{
		Kind: "binary_not_refreshed",
		Code: 1,
		Message: fmt.Sprintf("The installation was moved to %s, but its compiled command could not be obtained (%s). "+
			"%s is still the previous build, which may not match the files it now runs over", now, strings.Join(reasons, "; "), target),
		Hint: fmt.Sprintf("Install Go (https://go.dev/dl/) and run: cd %s && go build -o bin/hm ./cmd/hm"+
			" — or pin a release, whose binary is downloaded: %s switch --list", root, e.options.Binary),
	}
}

// releaseBinary downloads asset of the release tag and checks it against the release's
// checksums.txt. Fail closed: no manifest, no line for the asset, or a different digest is no
// binary.
func releaseBinary(source binarySource, tag, asset string) ([]byte, error) {
	binary, err := source.download(releaseDownloads + "/" + tag + "/" + asset)
	if err != nil {
		return nil, fmt.Errorf("downloading %s of %s failed: %v", asset, tag, err)
	}

	manifest, err := source.download(releaseDownloads + "/" + tag + "/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("downloading the checksums of %s failed: %v", tag, err)
	}

	want := ""
	scanner := bufio.NewScanner(bytes.NewReader(manifest))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			want = strings.ToLower(fields[0])
		}
	}

	if want == "" {
		return nil, fmt.Errorf("the checksums of %s do not list %s", tag, asset)
	}

	digest := sha256.Sum256(binary)
	if got := hex.EncodeToString(digest[:]); got != want {
		return nil, fmt.Errorf("%s of %s does not match its checksum (sha256 %s, expected %s)", asset, tag, got, want)
	}

	return binary, nil
}

// replaceWith has write produce the new file at a temporary path beside target, makes it
// executable and renames it over target. Whatever fails, target is as it was.
func replaceWith(target string, write func(path string) error) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(dir, ".hm-new-*")
	if err != nil {
		return err
	}
	path := temporary.Name()
	_ = temporary.Close()

	done := false
	defer func() {
		if !done {
			_ = os.Remove(path)
		}
	}()

	if err := write(path); err != nil {
		return err
	}

	if info, err := os.Stat(path); err != nil {
		return err
	} else if info.Size() == 0 {
		return errors.New("the new binary is empty")
	}

	if err := os.Chmod(path, 0o755); err != nil {
		return err
	}

	if err := os.Rename(path, target); err != nil {
		return err
	}

	done = true

	return nil
}

func copyFile(from, to string) error {
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}

	return os.WriteFile(to, content, 0o755)
}
