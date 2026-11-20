package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What `ssl` does: mkcert found or installed on this machine, a certificate generated into a
// temporary directory, its pem copied into hitch, chowned there as root, and hitch restarted. The
// host programs, the copy and Compose all land in one ordered log, so each test compares a single
// sequence. Nothing on this machine is ever run: the programs are a fake that writes what mkcert
// would have written.
//

// programs is this machine as a test describes it: which programs are on the PATH, which ones
// appear once another has run (mkcert, after its installation), and which ones fail.
type programs struct {
	log *[]string

	present  map[string]bool
	installs map[string]string
	fails    map[string]bool

	// dirs is every directory a program was run in.
	dirs []string
}

func (p *programs) LookPath(name string) (string, error) {
	*p.log = append(*p.log, "look "+name)

	if p.present[name] {
		return "/usr/local/bin/" + name, nil
	}

	return "", errors.New(name + " not found")
}

func (p *programs) Run(dir, name string, args ...string) error {
	entry := strings.TrimSpace(name + " " + strings.Join(args, " "))
	*p.log = append(*p.log, "host "+entry)
	p.dirs = append(p.dirs, dir)

	if p.fails[name] {
		return errors.New("exit status 1")
	}

	if installed, ok := p.installs[entry]; ok {
		p.present[installed] = true
	}

	// What the download leaves, under a name carrying its version
	if name == "curl" {
		if err := os.WriteFile(filepath.Join(dir, "mkcert-v1.4.4-linux-amd64"), []byte("binary"), 0o600); err != nil {
			return err
		}
	}

	// What mkcert writes when it generates: the certificate and the key where it was told to
	if name == "mkcert" && len(args) > 4 && args[0] == "-cert-file" {
		if err := os.WriteFile(args[1], []byte("CERTIFICATE\n"), 0o600); err != nil {
			return err
		}

		if err := os.WriteFile(args[3], []byte("KEY\n"), 0o600); err != nil {
			return err
		}
	}

	return nil
}

// certificateFiles logs every copy into a container and keeps what was copied, read at the moment
// of the copy: the directory it came from is gone by the time the test looks.
type certificateFiles struct {
	log    *[]string
	copied string
}

func (f *certificateFiles) Into(container, source, target string) error {
	*f.log = append(*f.log, "into "+container+" "+filepath.Base(source)+" "+target)

	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	f.copied = string(contents)

	return nil
}

func (f *certificateFiles) From(string, string, string) error { return nil }

var certificateProject = core.Project{Name: "tienda", Root: "/code/tienda"}

// certificateOperator is an operator on macOS over a running hitch, whose host has the programs
// given and nothing else.
func certificateOperator(log *[]string, present ...string) (Operator, *programs, *certificateFiles) {
	host := &programs{log: log, present: map[string]bool{}, installs: map[string]string{}, fails: map[string]bool{}}
	for _, name := range present {
		host.present[name] = true
	}

	copies := &certificateFiles{log: log}

	operator := operatorWith(runningLinuxStack("hitch"), &orchestrator{log: log})
	operator.Programs = host
	operator.Files = copies

	return operator, host, copies
}

// temporaryRoot points os.MkdirTemp at a directory of the test's own, and answers it.
func temporaryRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	t.Setenv("TMPDIR", root)

	return root
}

func assertNothingLeftIn(t *testing.T, root string) {
	t.Helper()

	left, err := filepath.Glob(filepath.Join(root, "hm-ssl-*"))
	if err != nil {
		t.Fatal(err)
	}

	if len(left) != 0 {
		t.Errorf("left behind: %v, want the temporary directory removed", left)
	}
}

// generation is what generating and installing logs once mkcert is there, for domain, with the
// certificate files named under work.
func generation(work, domain string) []string {
	return []string{
		"host mkcert -cert-file " + filepath.Join(work, "ssl.crt") + " -key-file " +
			filepath.Join(work, "ssl.key") + " " + domain + " localhost 127.0.0.1 0.0.0.0 ::1",
		"into hitch-1 testcert.pem /etc/hitch/",
		execEntry("root", "hitch", "chown", "hitch", "/etc/hitch/testcert.pem"),
		"restart hitch",
	}
}

// restarting is the restart the engine gives: Compose's own, logged.
func restarting(log *[]string) func() error {
	return func() error {
		*log = append(*log, "restart hitch")

		return nil
	}
}

func TestACertificateIsGeneratedCopiedChownedAndHitchRestarted(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	announced := []string{}
	operator, host, copies := certificateOperator(&log, "mkcert")
	operator.Announce = func(message string) { announced = append(announced, message) }

	if err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log)); err != nil {
		t.Fatalf("InstallCertificate: %v", err)
	}

	if len(host.dirs) != 1 || filepath.Dir(host.dirs[0]) != root {
		t.Fatalf("mkcert ran in %v, want one temporary directory under %s", host.dirs, root)
	}

	want := append([]string{"look mkcert"}, generation(host.dirs[0], "tienda.local")...)
	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("what happened (-want +got):\n%s", diff)
	}

	if copies.copied != "CERTIFICATE\nKEY\n" {
		t.Errorf("pem copied = %q, want the certificate followed by its key", copies.copied)
	}

	assertNothingLeftIn(t, root)

	wantAnnounced := []string{
		"Generating SSL certificates for domain tienda.local...\n",
		"Installing SSL certificate into docker environment...\n",
		"SSL certificate installed. Remember to restart your browser\n",
	}
	if diff := cmp.Diff(wantAnnounced, announced); diff != "" {
		t.Errorf("announced (-want +got):\n%s", diff)
	}
}

func TestMkcertIsInstalledWithBrewOnMacOS(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, host, _ := certificateOperator(&log, "brew")
	host.installs["brew install mkcert nss"] = "mkcert"

	if err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log)); err != nil {
		t.Fatalf("InstallCertificate: %v", err)
	}

	want := append([]string{
		"look mkcert",
		"look brew",
		"host brew install mkcert nss",
		"host mkcert -install",
		"look mkcert",
	}, generation(host.dirs[0], "tienda.local")...)

	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("what happened (-want +got):\n%s", diff)
	}

	assertNothingLeftIn(t, root)
}

func TestWithoutBrewOnMacOSNothingIsGenerated(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, _, _ := certificateOperator(&log)

	err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log))

	if refusal := refusalOf(t, err); refusal.Code != 1 || refusal.Message != "Brew is required. Please install it and try again." {
		t.Errorf("refusal = %+v, want code 1 asking for brew", refusal)
	}

	if diff := cmp.Diff([]string{"look mkcert", "look brew"}, log); diff != "" {
		t.Errorf("what happened (-want +got):\n%s", diff)
	}

	assertNothingLeftIn(t, root)
}

func TestMkcertIsDownloadedAndInstalledOnLinux(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, host, _ := certificateOperator(&log)
	operator.Platform = "linux"
	host.installs["sudo mv mkcert-v1.4.4-linux-amd64 /usr/local/bin/mkcert"] = "mkcert"

	// Stopped right after generating: what follows is the same on every platform, and on Linux the
	// restart would go on to the steps after start, which are after_start_test.go's
	operator.Files = failingFiles{}

	err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log))
	if err == nil || err.Error() != "copy refused" {
		t.Fatalf("InstallCertificate = %v, want it stopped at the copy", err)
	}

	work := host.dirs[0]

	want := []string{
		"look mkcert",
		"host sudo apt-get -y install curl libnss3-tools",
		"host curl -JLO https://dl.filippo.io/mkcert/latest?for=linux/amd64",
		"host chmod +x mkcert-v1.4.4-linux-amd64",
		"host sudo mv mkcert-v1.4.4-linux-amd64 /usr/local/bin/mkcert",
		"host mkcert -install",
		"look mkcert",
		generation(work, "tienda.local")[0],
	}
	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("what happened (-want +got):\n%s", diff)
	}

	for _, dir := range host.dirs {
		if dir != work {
			t.Errorf("a program ran in %s, want every one in the download directory %s", dir, work)
		}
	}

	assertNothingLeftIn(t, root)
}

type failingFiles struct{}

func (failingFiles) Into(string, string, string) error { return errors.New("copy refused") }
func (failingFiles) From(string, string, string) error { return nil }

func TestMkcertStillMissingAfterItsInstallationIsRefused(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, _, _ := certificateOperator(&log, "brew")

	err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log))

	refusal := refusalOf(t, err)
	if refusal.Code != 1 || refusal.Message != "Error during 'mkcert' installation. Please do it manually and try again..." {
		t.Errorf("refusal = %+v, want code 1 for a failed installation", refusal)
	}

	if log[len(log)-1] != "look mkcert" {
		t.Errorf("last = %q, want nothing generated after mkcert was looked for again", log[len(log)-1])
	}

	assertNothingLeftIn(t, root)
}

func TestWithoutHitchRunningNothingHappensOnTheHost(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, _, _ := certificateOperator(&log, "mkcert")
	operator.Engine = &engine{containers: runningLinuxStack("nginx")}

	err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log))

	refusal := refusalOf(t, err)
	if refusal.Code != 5 || refusal.Kind != "service_not_running" || refusal.Message != "Service 'hitch' is not running" {
		t.Errorf("refusal = %+v, want 5 service_not_running for hitch", refusal)
	}

	if len(log) != 0 {
		t.Errorf("what happened = %v, want nothing", log)
	}

	assertNothingLeftIn(t, root)
}

func TestAFailingMkcertStopsBeforeAnythingIsCopied(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, host, _ := certificateOperator(&log, "mkcert")
	host.fails["mkcert"] = true

	err := operator.InstallCertificate(certificateProject, core.ComposeFiles{}, "tienda.local", restarting(&log))
	if refusal := refusalOf(t, err); refusal.Code != 1 {
		t.Errorf("refusal = %+v, want code 1", refusal)
	}

	want := []string{"look mkcert", generation(host.dirs[0], "tienda.local")[0]}
	if diff := cmp.Diff(want, log); diff != "" {
		t.Errorf("what happened (-want +got):\n%s", diff)
	}

	assertNothingLeftIn(t, root)
}

func TestFromAWorktreeWithNoEnvironmentOfItsOwnSslIsRefused(t *testing.T) {
	root := temporaryRoot(t)
	log := []string{}
	operator, _, _ := certificateOperator(&log, "mkcert")
	project := certificateProject
	project.InWorktree = true

	err := operator.InstallCertificate(project, core.ComposeFiles{}, "tienda.local", restarting(&log))

	if refusal := refusalOf(t, err); refusal.Kind != "blocked_in_worktree" || refusal.Code != 6 {
		t.Errorf("refusal = %+v, want 6 blocked_in_worktree", refusal)
	}

	if len(log) != 0 {
		t.Errorf("what happened = %v, want nothing", log)
	}

	assertNothingLeftIn(t, root)
}
