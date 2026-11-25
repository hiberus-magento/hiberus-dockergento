package dockergento

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The tool is one self-contained binary, and it moves between versions by replacing itself with
// the binary of another release.
//
// Releases are GitHub's: every tag of 2.0 or later publishes hm_<os>_<arch> for each platform and a
// checksums.txt with their sha256. `hm update` and `hm switch` read the list of releases, download
// the binary of the one they move to and its checksums, check one against the other, and rename the
// new binary over the running executable. Fail closed: no manifest, no line for the asset or a
// different digest is no binary, and the running executable stays as it was.
//
// The replacement is atomic — a temporary file beside the executable, made executable, renamed over
// it — and renaming over the running executable is safe on Linux and macOS: the process keeps the
// inode it was started from, and nothing here executes the path again.
//
// The executable's links are resolved first, so /usr/local/bin/hm linking to ~/.hm/bin/hm — what
// the installer leaves when /usr/local/bin is root's — is replaced at ~/.hm/bin/hm, without root.
// A binary sitting in a directory only root writes (2.0.0-rc.1 to rc.3 were installed that way) is
// moved: with a person at a terminal, the new binary is written to ~/.hm/bin/hm and sudo, once,
// makes the old path a link to it. With nobody to give sudo a password, the update is refused.
//

// releasesAPI lists the tool's releases, newest first.
const releasesAPI = "https://api.github.com/repos/hiberus-magento/hiberus-dockergento/releases"

// releasePage is where a person reads what a release changed.
const releasePage = "https://github.com/hiberus-magento/hiberus-dockergento/releases/tag/"

// repository is where the tool is published, and where a 1.x or a branch is cloned from.
const repository = "https://github.com/hiberus-magento/hiberus-dockergento.git"

// ReleasesURLVariable replaces the address the releases are listed from. It is for test suites that
// run `hm update` or `hm switch` for real and must not reach the network: they point it at a
// file:// fixture in the GitHub API's format, whose assets are file:// URLs too. Unset or empty, the
// releases are GitHub's.
const ReleasesURLVariable = "HM_RELEASES_URL"

// release is one release as GitHub lists it.
type release struct {
	Tag        string
	PreRelease bool
	Draft      bool

	// Assets is each published file's download URL by its name.
	Assets map[string]string
}

// releaseSource is the port releases are read through. Injected so tests never reach the network.
type releaseSource interface {
	// Releases lists every release published.
	Releases() ([]release, error)

	// Download returns the body of an asset's URL, or an error for anything but a whole 200.
	Download(url string) ([]byte, error)
}

// installer is how a release is put in place of the running executable.
type installer struct {
	source releaseSource

	// executable is the path of the running executable, as it was started: through a link,
	// possibly, which the replacement resolves.
	executable func() (string, error)

	goos, goarch string

	// interactive is whether a person at a terminal is there to give root's password, which is
	// what moving a binary out of a directory only root writes takes.
	interactive func() bool

	// home is the user's home directory, where ~/.hm/bin is.
	home func() (string, error)

	// sudo runs a command as root, attached to the terminal so that sudo can ask for the
	// password; nil when there is no sudo.
	sudo func(args ...string) error

	// notice is where the move to ~/.hm/bin says what it does, on its way and once done.
	notice io.Writer
}

// realInstaller is GitHub, or the fixture ReleasesURLVariable names, and the executable of this
// process.
func realInstaller() installer {
	address := releasesAPI
	if fixture := os.Getenv(ReleasesURLVariable); fixture != "" {
		address = fixture
	}

	install := installer{
		source:     githubReleases{address: address, token: os.Getenv("GITHUB_TOKEN")},
		executable: os.Executable,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		interactive: func() bool {
			return inputIsTerminal() && os.Getenv("HM_NON_INTERACTIVE") == ""
		},
		home:   os.UserHomeDir,
		notice: os.Stderr,
	}

	if sudo, err := exec.LookPath("sudo"); err == nil {
		install.sudo = func(args ...string) error {
			command := exec.Command(sudo, args...)
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr

			return command.Run()
		}
	}

	return install
}

// inputIsTerminal is whether this process reads from a terminal. A variable so tests decide.
var inputIsTerminal = func() bool { return terminal(os.Stdin) }

// newInstaller is what an engine without an injected installer uses. A variable so the package's
// tests can replace it for every engine they build.
var newInstaller = realInstaller

func (e *Engine) installer() installer {
	if e.install != nil {
		return *e.install
	}

	return newInstaller()
}

// target is the file the running executable really is, links resolved: /usr/local/bin/hm linking
// to ~/.hm/bin/hm is replaced at ~/.hm/bin/hm, and the link is kept.
func (i installer) target() (string, error) {
	executable, err := i.executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(executable)
}

func runningExecutable() (string, error) {
	return installer{executable: os.Executable}.target()
}

// githubReleases reads the releases API, unauthenticated unless GITHUB_TOKEN is set. A file:// address
// is read from disk, which is what ReleasesURLVariable is for.
type githubReleases struct {
	address, token string
}

func (g githubReleases) Releases() ([]release, error) {
	address := g.address
	if strings.HasPrefix(address, "http") {
		// One page of 100 is every release for years; the API's default page is 30
		address += "?per_page=100"
	}

	body, err := g.fetch(address, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}

	var listed []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("the list of releases is not what GitHub answers: %v", err)
	}

	releases := make([]release, 0, len(listed))
	for _, item := range listed {
		assets := map[string]string{}
		for _, asset := range item.Assets {
			assets[asset.Name] = asset.URL
		}

		releases = append(releases, release{Tag: item.TagName, PreRelease: item.Prerelease, Draft: item.Draft, Assets: assets})
	}

	return releases, nil
}

func (g githubReleases) Download(address string) ([]byte, error) {
	return g.fetch(address, "application/octet-stream")
}

func (g githubReleases) fetch(address, accept string) ([]byte, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if parsed.Scheme == "file" {
		return os.ReadFile(parsed.Path)
	}

	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", accept)

	// The token is GitHub's and goes to GitHub only, never to an address somebody configured; Go
	// drops it by itself on a redirect to another host, which is where assets are served from
	if g.token != "" && (parsed.Host == "api.github.com" || parsed.Host == "github.com") {
		request.Header.Set("Authorization", "Bearer "+g.token)
	}

	client := http.Client{Timeout: 5 * time.Minute}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if (response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests) &&
			response.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("GitHub's rate limit was reached (%s); set GITHUB_TOKEN to raise it", response.Status)
		}

		return nil, fmt.Errorf("%s answered %s", address, response.Status)
	}

	// A binary is tens of megabytes; anything past this is not one
	return io.ReadAll(io.LimitReader(response.Body, 512<<20))
}

// releaseAsset is the asset name of this platform, or "" when releases do not publish one for it.
func releaseAsset(goos, goarch string) string {
	if (goos != "darwin" && goos != "linux") || (goarch != "amd64" && goarch != "arm64") {
		return ""
	}

	return "hm_" + goos + "_" + goarch
}

//
// Versions. Tags have no "v" (2.0.0, 2.0.0-rc.1), and are compared as semantic versions: a
// pre-release comes before its final release, and its identifiers compare numerically when they are
// numbers, so rc.10 comes after rc.9.
//

type semver struct {
	major, minor, patch int
	pre                 []string
}

// parseVersion reads X.Y.Z with an optional -pre-release, a leading "v" tolerated and build
// metadata ignored.
func parseVersion(text string) (semver, bool) {
	text = strings.TrimPrefix(text, "v")
	text, _, _ = strings.Cut(text, "+")
	base, pre, hasPre := strings.Cut(text, "-")

	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return semver{}, false
	}

	numbers := [3]int{}
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || part == "" {
			return semver{}, false
		}
		numbers[i] = number
	}

	version := semver{major: numbers[0], minor: numbers[1], patch: numbers[2]}

	if hasPre {
		if pre == "" {
			return semver{}, false
		}
		version.pre = strings.Split(pre, ".")
	}

	return version, true
}

// less is whether a comes before b.
func (a semver) less(b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	if a.patch != b.patch {
		return a.patch < b.patch
	}

	switch {
	case len(a.pre) == 0:
		return false
	case len(b.pre) == 0:
		return true
	}

	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		if x == y {
			continue
		}

		xn, xErr := strconv.Atoi(x)
		yn, yErr := strconv.Atoi(y)

		switch {
		case xErr == nil && yErr == nil:
			return xn < yn
		case xErr == nil:
			return true
		case yErr == nil:
			return false
		default:
			return x < y
		}
	}

	return len(a.pre) < len(b.pre)
}

// published is a release of the Go tool, with its version read.
type published struct {
	release
	version semver
}

// releases is every release of 2.0 or later, newest first: no drafts, and no pre-releases unless
// asked for. 1.x is the shell tool, which has no binary to install.
func (e *Engine) releases(source releaseSource, preReleases bool) ([]published, error) {
	listed, err := source.Releases()
	if err != nil {
		return nil, core.Refusal{
			Kind:    "releases_unavailable",
			Code:    contract.ExitError,
			Message: "Could not read the releases of Hiberus Dockergento: " + err.Error(),
			Hint:    "Check the connection to GitHub and try again; if GitHub limits you, set GITHUB_TOKEN",
		}
	}

	found := []published{}
	for _, item := range listed {
		version, ok := parseVersion(item.Tag)
		if !ok || version.major < 2 || item.Draft || (item.PreRelease && !preReleases) {
			continue
		}

		found = append(found, published{release: item, version: version})
	}

	sort.SliceStable(found, func(i, j int) bool { return found[j].version.less(found[i].version) })

	return found, nil
}

// runningRelease is the running version read, or a refusal for a binary that is not a release: one
// built from a checkout has no version to compare and is updated where it was built.
//
// A stamp in the shape `git describe --tags` prints (2.0.0-5-gdeadbee, -dirty) or a GoReleaser
// snapshot's (2.0.1-SNAPSHOT-deadbee) is a source build too: read as semver it is a pre-release
// sorting below its tag, and update would "upgrade" a build past 2.0.0 to the older 2.0.0.
func (e *Engine) runningRelease(running string) (semver, error) {
	if version, ok := parseVersion(running); ok && version.major >= 2 && !sourceStamp.MatchString(running) {
		return version, nil
	}

	return semver{}, core.Refusal{
		Kind: "built_from_source",
		Code: contract.ExitBlocked,
		Message: fmt.Sprintf("This %s was built from a checkout (version %s), not installed from a release, "+
			"so it is not replaced with one", e.options.Binary, orDevel(running)),
		Hint: "Update it where it was built: git pull && go build -o bin/hm ./cmd/hm",
	}
}

// sourceStamp matches the stamps of builds that are not releases: the commits past a tag that git
// describe adds, a dirty tree, or a snapshot.
var sourceStamp = regexp.MustCompile(`(?i)-[0-9]+-g[0-9a-f]{4,}(-dirty)?$|-dirty$|snapshot`)

func orDevel(version string) string {
	if version == "" {
		return "devel"
	}

	return version
}

// put downloads the binary of release for this platform, checks it against the release's
// checksums.txt and renames it over the running executable, whose path it returns. Whatever fails,
// the executable is as it was.
func (e *Engine) put(install installer, item published) (string, error) {
	tag := item.Tag
	unchanged := func(target string) string {
		if target == "" {
			return "the running executable was not changed"
		}

		return target + " was not changed"
	}

	asset := releaseAsset(install.goos, install.goarch)
	address := item.Assets[asset]

	if asset == "" || address == "" {
		return "", core.Refusal{
			Kind: "asset_missing",
			Code: contract.ExitError,
			Message: fmt.Sprintf("Release %s publishes no binary for %s/%s; %s",
				tag, install.goos, install.goarch, unchanged("")),
			Hint: "Build it from source: git clone -b " + tag + " " + repository +
				" && cd hiberus-dockergento && go build -o hm ./cmd/hm",
		}
	}

	target, err := install.target()
	if err != nil {
		return "", core.Refusal{
			Kind:    "install_failed",
			Code:    contract.ExitError,
			Message: "Could not find the running executable to replace it: " + err.Error(),
			Hint:    "Re-run the installer to install " + tag,
		}
	}

	// The temporary file comes first: a directory that cannot be written is said before anything is
	// downloaded
	dir := filepath.Dir(target)

	temporary, err := os.CreateTemp(dir, ".hm-new-*")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			if moved, ok := install.movable(target); ok {
				return e.moveHome(install, item, asset, target, moved)
			}
		}

		return "", e.notWritable(target, tag, err)
	}
	path := temporary.Name()
	_ = temporary.Close()

	done := false
	defer func() {
		if !done {
			_ = os.Remove(path)
		}
	}()

	binary, err := verifiedAsset(install.source, item, asset)
	if err != nil {
		return "", core.Refusal{
			Kind:    "download_failed",
			Code:    contract.ExitError,
			Message: err.Error() + "; " + unchanged(target),
			Hint:    "Try again; if it keeps failing, re-run the installer",
		}
	}

	if err := os.WriteFile(path, binary, 0o755); err != nil {
		return "", e.notWritable(target, tag, err)
	}

	if err := os.Chmod(path, 0o755); err != nil {
		return "", e.notWritable(target, tag, err)
	}

	if err := os.Rename(path, target); err != nil {
		return "", e.notWritable(target, tag, err)
	}

	done = true

	return target, nil
}

// installScript is the installer, which installs the newest release and moves a binary root
// installed to ~/.hm/bin.
const installScript = "https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh"

// notWritableHint is what a user who cannot write the executable's directory can do instead.
const notWritableHint = "Re-run the installer, which asks for sudo once to move hm to ~/.hm/bin so that " +
	"updates need none: curl -fsSL " + installScript + " | bash; or install where you can write: " +
	"curl -fsSL " + installScript + " | HM_INSTALL_DIR=$HOME/.local/bin bash"

// movable is where the binary goes when its directory is root's, ~/.hm/bin/hm, and whether it may
// go there now: only with a person at a terminal to give sudo the password. Without one (CI, a
// pipe, --json, HM_NON_INTERACTIVE) or without sudo, the replacement is refused as it always was.
func (i installer) movable(target string) (string, bool) {
	if i.interactive == nil || !i.interactive() || i.sudo == nil || i.home == nil {
		return "", false
	}

	home, err := i.home()
	if err != nil || home == "" {
		return "", false
	}

	moved := filepath.Join(home, ".hm", "bin", "hm")

	// ~/.hm/bin itself not writable is nothing a link to it fixes
	return moved, moved != target
}

// moveHome installs item at moved, ~/.hm/bin/hm, and has sudo make target a link to it: root once,
// for a binary root installed (2.0.0-rc.1 to rc.3 put it in /usr/local/bin), and never again,
// because the next update replaces ~/.hm/bin/hm through the link. Until sudo has linked it, target
// is the binary it was and still runs, and a copy in ~/.hm/bin that nothing points at is harmless.
func (e *Engine) moveHome(install installer, item published, asset, target, moved string) (string, error) {
	tag := item.Tag
	dir := filepath.Dir(target)

	binary, err := verifiedAsset(install.source, item, asset)
	if err != nil {
		return "", core.Refusal{
			Kind:    "download_failed",
			Code:    contract.ExitError,
			Message: err.Error() + "; " + target + " was not changed",
			Hint:    "Try again; if it keeps failing, re-run the installer",
		}
	}

	notice := install.notice
	if notice == nil {
		notice = io.Discard
	}

	fmt.Fprintf(notice, "%s is in %s, which only root can write: sudo is needed this once, to move hm to %s "+
		"and leave a link to it in its place\n", target, dir, filepath.Dir(moved))

	if err := placeAt(moved, binary); err != nil {
		return "", core.Refusal{
			Kind:    "install_failed",
			Code:    contract.ExitError,
			Message: fmt.Sprintf("Could not write %s to %s: %v; %s was not changed", tag, moved, err, target),
			Hint:    notWritableHint,
		}
	}

	if err := install.sudo("ln", "-sfn", moved, target); err != nil {
		return "", core.Refusal{
			Kind: "install_dir_not_writable",
			Code: contract.ExitError,
			Message: fmt.Sprintf("Cannot replace %s: %s is not writable by you and sudo did not link it (%v), "+
				"so %s was not installed; %s still runs as before, and %s is a copy nothing uses yet",
				target, dir, err, tag, target, moved),
			Hint: notWritableHint,
		}
	}

	fmt.Fprintf(notice, "hm now lives in %s and %s links to it, so later updates need no sudo\n", moved, target)

	return moved, nil
}

// placeAt writes binary to path in one rename, creating its directory: a temporary file beside it,
// made executable and renamed over whatever was there.
func placeAt(path string, binary []byte) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(dir, ".hm-new-*")
	if err != nil {
		return err
	}
	name := temporary.Name()

	_, err = temporary.Write(binary)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, 0o755)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}

	return err
}

// notWritable is a replacement the directory of the executable did not allow: installed by root, in
// /usr/local/bin say, with nobody at a terminal to give sudo the password. The installer can move
// it to ~/.hm/bin with the rights it needs, or put it somewhere the user owns.
func (e *Engine) notWritable(target, tag string, err error) error {
	dir := filepath.Dir(target)

	if errors.Is(err, fs.ErrPermission) {
		return core.Refusal{
			Kind: "install_dir_not_writable",
			Code: contract.ExitError,
			Message: fmt.Sprintf("Cannot replace %s: %s is not writable by you, so %s was not installed",
				target, dir, tag),
			Hint: notWritableHint,
		}
	}

	return core.Refusal{
		Kind:    "install_failed",
		Code:    contract.ExitError,
		Message: fmt.Sprintf("Could not replace %s with %s: %v; it was not changed", target, tag, err),
		Hint:    "Re-run the installer to install " + tag,
	}
}

// verifiedAsset downloads asset of item and checks it against the release's checksums.txt.
func verifiedAsset(source releaseSource, item published, asset string) ([]byte, error) {
	tag := item.Tag

	manifestURL := item.Assets["checksums.txt"]
	if manifestURL == "" {
		return nil, fmt.Errorf("release %s publishes no checksums.txt to verify %s with", tag, asset)
	}

	binary, err := source.Download(item.Assets[asset])
	if err != nil {
		return nil, fmt.Errorf("downloading %s of %s failed: %v", asset, tag, err)
	}

	manifest, err := source.Download(manifestURL)
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

	if len(binary) == 0 {
		return nil, fmt.Errorf("%s of %s is empty", asset, tag)
	}

	return binary, nil
}
