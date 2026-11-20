package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// wrappers_test.go asks these questions of what the copy and varnish handlers reach for. The
// fake below is what answers them: an ordered log of what was asked, and — where a test needs
// one — an outcome to answer it with.
//

// call is one thing asked of a fakeEngine, in the order it was asked. Every commands method
// records one, so a whole test compares against a single log rather than per-method slices,
// which is what makes an interleaving like varnish's own visible in a diff.
type call struct {
	Method   string
	Dir      string
	Service  string
	Command  []string
	Options  core.ExecOptions
	Paths    []string
	All      bool
	Services []string
	Key      string
	Path     string
	Domain   string
	Database bool

	// Port is the port share was asked to tunnel to, and Confirm what its question was answered.
	Port    int
	Confirm bool

	// Snapshot is whether stop was asked to save the database first.
	Snapshot bool

	// StopOthers is whether start was asked to stop every other environment first (-s).
	StopOthers bool

	// DownOptions is what down was asked for, compared whole.
	DownOptions core.DownOptions

	// Statement is what mysql -q was asked to run.
	Statement string

	// ImportOptions is what mysql -i was asked to import, compared whole.
	ImportOptions app.ImportOptions

	// SetupOptions is what setup was asked for, compared whole.
	SetupOptions core.SetupOptions

	// CreateProjectOptions is what create-project was asked for, compared whole.
	CreateProjectOptions core.CreateProjectOptions

	// Interactive is what docker-stop-all was asked with: whether the stop may ask before doing
	// it.
	Interactive bool

	// WithSecrets is whether describe was asked for the credentials. It is the only argument of
	// that call besides the directory, and it is the one worth a regression: false by default is
	// what keeps a database password out of a description nobody asked to contain one.
	WithSecrets bool

	// LogOptions is what logs was asked for, compared whole: a flag that starts writing a
	// different field shows up as a diff here rather than nothing.
	LogOptions core.LogOptions

	// Only is the single check doctor was asked to run (--only), "" for all of them.
	Only string

	// VerifyOptions is what verify was asked to check, compared whole.
	VerifyOptions core.VerifyOptions

	// Collection is what clean asked to be collected, compared whole: what is deleted has to be
	// exactly what the survey found, and a field that drifts shows up as a diff.
	Collection core.Collection
}

// outcome is what a call answers with. outcomes is indexed by call number — the position the
// call lands at in the log — and a call past the end of outcomes succeeds, so a test states only
// the calls that must fail.
type outcome struct {
	status int
	err    error

	// captured is what Capture or XdebugHost answers with, on the same terms Resolve answers
	// from f.project: per-call, not per-method. debug-on calls Capture twice — php -v, then
	// php -i — and needs a different string back each time; outcomes is already indexed by call
	// number, so a field here gives that for free, where a field on fakeEngine itself could not.
	captured string

	// diagnosis is what Diagnose answers with, per call like captured: doctor asks once, and each
	// test states the findings it wants reported.
	diagnosis core.Diagnosis

	// collection is what Survey answers with, per call like diagnosis: clean surveys once, and
	// each test states what it wants found.
	collection core.Collection

	// files is what DefaultMagentoFiles answers with, per call like collection: composer's macOS
	// dance asks once, and each test states the list it wants copied in.
	files []string
}

// fakeEngine is a commands that never reaches Docker: everything asked of it lands in calls, and
// nothing beyond outcomes decides what it answers.
type fakeEngine struct {
	calls    []call
	outcomes []outcome

	// project is what Resolve answers with, when its own outcome does not fail it. Left at its
	// zero value, Name is empty — which is what a directory with no project looks like to the
	// handlers that call it.
	project core.Project

	// properties is what Property answers from, keyed the same way the real engine's own
	// properties are. A key left out of the map answers "", which is what drives the fallbacks
	// wrappers.go falls back to when a project never set one.
	properties map[string]string

	// installed and tooling are what Installed answers with — the other thing asked of the
	// engine that only ever answers, never fails, the same as Property.
	installed core.Installation
	tooling   core.Tooling

	// stopped is what StopEverything answers from, when its own outcome does not fail it — the
	// same shape as project for Resolve.
	stopped core.MachineStop

	// environments is what Environments answers with, when its own outcome does not fail it — the
	// same shape as project for Resolve. Environments takes no directory, so there is nothing
	// else to answer from.
	environments []core.Environment

	// description is what Describe answers with, on the same terms.
	description core.Description

	// share is what Share answers with, and unshared what Unshare does.
	share    core.Share
	unshared bool

	// compatibility is what Compatibility answers with.
	compatibility string

	// equivalents is what Equivalents answers with, and settings what InstallSettings does —
	// SaveInstallSettings stores into it, so a later read sees what was saved, as with the file.
	equivalents map[string]string
	settings    []core.Setting

	// byCommand, when set, answers Exec and Capture by the command they were given rather than by
	// call number, for a test whose calls land at positions that depend on earlier answers.
	byCommand func(command []string) (outcome, bool)

	// dockerDown is the answer DockerReachable gives, negated so the zero value is a daemon up.
	dockerDown bool

	// verification is what Verify answers with.
	verification core.Verification

	// versions is what Versions answers with, switched what Switch does and updated what Update
	// does. switchOptions, updateOptions and versionsRunning are what they were asked.
	versions        core.Versions
	switched        core.Switched
	updated         core.Updated
	switchOptions   core.SwitchOptions
	updateOptions   core.UpdateOptions
	versionsRunning string
	// aiDoctor is what AIDoctor answers with.
	aiDoctor core.AIDoctorReport

	// aiTools is the use case AIInit, AIPull and AIReset run, against real directories: the
	// tool's installation and the project are temporary ones, and downloads go through aiExec.
	aiTools *app.AITools

	// tunnel is what Tunnel answers with, and tunnelsClosed what CloseTunnels does.
	tunnel        core.Tunnel
	tunnelsClosed bool

	// permissions is what AgentPermissions answers with.
	permissions core.AgentPermissions

	// agentContext is what WriteAgentContext answers with, when its own outcome does not fail it.
	agentContext core.AgentContextResult
}

var _ commands = (*fakeEngine)(nil)

func (f *fakeEngine) outcomeFor(number int) outcome {
	if number < len(f.outcomes) {
		return f.outcomes[number]
	}

	return outcome{}
}

func (f *fakeEngine) Resolve(dir string) (core.Project, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Resolve", Dir: dir})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Project{}, out.err
	}

	return f.project, nil
}

// Environments, like Resolve, answers from a field and can still be refused: an unreachable
// daemon is what list.go:24 branches on. It takes no directory, so the call records only its
// name — the same single-field entry version's Installed leaves.
func (f *fakeEngine) Environments() ([]core.Environment, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Environments"})

	if out := f.outcomeFor(number); out.err != nil {
		return nil, out.err
	}

	return f.environments, nil
}

// Describe answers from f.description on the same terms. WithSecrets is recorded rather than
// acted on: what the credentials are withheld from is app.Describer's decision, and this seam's
// job is to prove the flag arrives unchanged.
func (f *fakeEngine) Describe(dir string, withSecrets bool) (core.Description, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Describe", Dir: dir, WithSecrets: withSecrets})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Description{}, out.err
	}

	return f.description, nil
}

func (f *fakeEngine) Exec(dir, service string, command []string, options core.ExecOptions) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Exec", Dir: dir, Service: service, Command: command, Options: options})

	out := f.outcomeFor(number)
	if f.byCommand != nil {
		if answer, ok := f.byCommand(command); ok {
			out = answer
		}
	}

	return out.status, out.err
}

// AgentPermissions answers with the permissions it was given, and records whether strict was asked.
func (f *fakeEngine) AgentPermissions(strict bool) (core.AgentPermissions, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "AgentPermissions", All: strict})

	return f.permissions, f.outcomeFor(number).err
}

// WriteAgentContext answers from f.agentContext, and records the directory it was asked about.
func (f *fakeEngine) WriteAgentContext(dir string) (core.AgentContextResult, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "WriteAgentContext", Dir: dir})

	if out := f.outcomeFor(number); out.err != nil {
		return core.AgentContextResult{}, out.err
	}

	return f.agentContext, nil
}

// Update records what it was asked in updateOptions and answers with updated.
func (f *fakeEngine) Update(options core.UpdateOptions) (core.Updated, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Update"})
	f.updateOptions = options

	return f.updated, f.outcomeFor(number).err
}

// ConfigureForDevelopment only ever answers with an error.
func (f *fakeEngine) ConfigureForDevelopment(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "ConfigureForDevelopment", Dir: dir})

	return f.outcomeFor(number).err
}

// Setup only ever answers with an error.
func (f *fakeEngine) Setup(dir string, options core.SetupOptions, interactive bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Setup", Dir: dir, SetupOptions: options, Interactive: interactive})

	return f.outcomeFor(number).err
}

// CreateProject only ever answers with an error.
func (f *fakeEngine) CreateProject(dir string, options core.CreateProjectOptions, interactive bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CreateProject", Dir: dir, CreateProjectOptions: options, Interactive: interactive})

	return f.outcomeFor(number).err
}

// Query writes its outcome's captured string where the statement's output goes, and answers with
// its status, the way the client's own exit code comes back.
func (f *fakeEngine) Query(dir, statement string, out io.Writer) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Query", Dir: dir, Statement: statement})
	answer := f.outcomeFor(number)
	fmt.Fprint(out, answer.captured)

	return answer.status, answer.err
}

// Import only ever answers with an error.
func (f *fakeEngine) Import(dir string, options app.ImportOptions) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Import", Dir: dir, ImportOptions: options})

	return f.outcomeFor(number).err
}

// Feed and Console answer with the client's status.
func (f *fakeEngine) Feed(dir string) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Feed", Dir: dir})
	answer := f.outcomeFor(number)

	return answer.status, answer.err
}

func (f *fakeEngine) Console(dir string) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Console", Dir: dir})
	answer := f.outcomeFor(number)

	return answer.status, answer.err
}

// Down answers with its outcome's captured string as what happened: "" for nothing destroyed,
// "saved" for a copy taken first, anything else for destroyed.
func (f *fakeEngine) Down(dir string, options core.DownOptions, interactive bool) (string, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Down", Dir: dir, DownOptions: options, Interactive: interactive})
	answer := f.outcomeFor(number)

	return answer.captured, answer.err
}

// Start records the options whole: Services, StopOthers and Interactive.
func (f *fakeEngine) Start(dir string, options dockergento.StartOptions) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Start", Dir: dir, Services: options.Services,
		StopOthers: options.StopOthers, Interactive: options.Interactive})

	return f.outcomeFor(number).err
}

// Stop, like Restart, only ever answers with an error.
func (f *fakeEngine) Stop(dir string, services []string, snapshot bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Stop", Dir: dir, Services: services, Snapshot: snapshot})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) Restart(dir string, services []string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Restart", Dir: dir, Services: services})

	return f.outcomeFor(number).err
}

// AfterStart and Rebuild, like Restart, only ever answer with an error.
func (f *fakeEngine) AfterStart(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "AfterStart", Dir: dir})

	return f.outcomeFor(number).err
}

// InstallCertificate, like AfterStart, only ever answers with an error: what it generates and
// where it copies it is app.Operator.InstallCertificate's, and this seam proves the domain arrives.
func (f *fakeEngine) InstallCertificate(dir, domain string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "InstallCertificate", Dir: dir, Domain: domain})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) Rebuild(dir string, services []string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Rebuild", Dir: dir, Services: services})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) CopyInto(dir string, paths []string, all bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CopyInto", Dir: dir, Paths: paths, All: all})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) CopyFrom(dir string, paths []string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CopyFrom", Dir: dir, Paths: paths})

	return f.outcomeFor(number).err
}

// DefaultMagentoFiles answers from its outcome: the list, or the error that stopped the read. It
// asks about no project, so the call records only its name.
func (f *fakeEngine) DefaultMagentoFiles() ([]string, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "DefaultMagentoFiles"})

	out := f.outcomeFor(number)
	if out.err != nil {
		return nil, out.err
	}

	return out.files, nil
}

// ClearVendor, like CopyFrom, only ever answers with an error: what it deletes is the engine's
// decision, and this seam's job is to prove when it is asked and for which directory.
func (f *fakeEngine) ClearVendor(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "ClearVendor", Dir: dir})

	return f.outcomeFor(number).err
}

// Property answers from f.properties, not from an outcome: it is one of the two things asked of
// the engine that only ever answers, never fails — the same as the real engine, whose reader over
// two files answers "" for a project that never set the key.
func (f *fakeEngine) Property(project core.Project, key string) string {
	f.calls = append(f.calls, call{Method: "Property", Dir: project.Root, Key: key})

	return f.properties[key]
}

// Dump, unlike Property, is asked to do something and can be refused: it consumes an outcome the
// way Resolve and Exec do, rather than only ever answering.
func (f *fakeEngine) Dump(dir, path string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Dump", Dir: dir, Path: path})

	return f.outcomeFor(number).err
}

// Installed answers from f.installed and f.tooling, the same as Property: asked about no
// project, and never refused.
func (f *fakeEngine) Installed() (core.Installation, core.Tooling) {
	f.calls = append(f.calls, call{Method: "Installed"})

	return f.installed, f.tooling
}

// SetHost, like Dump, consumes an outcome and can be refused.
func (f *fakeEngine) SetHost(dir, domain string, database bool) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "SetHost", Dir: dir, Domain: domain, Database: database})

	return f.outcomeFor(number).err
}

// RemoveHost, unlike SetHost, asks nothing about a project: there is no dir to record.
func (f *fakeEngine) RemoveHost(domain string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "RemoveHost", Domain: domain})

	return f.outcomeFor(number).err
}

// StopEverything answers from f.stopped, like Resolve answers from f.project, and consumes an
// outcome for the error, like Dump and SetHost do.
func (f *fakeEngine) StopEverything(dir string, interactive bool) (core.MachineStop, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "StopEverything", Dir: dir, Interactive: interactive})

	if out := f.outcomeFor(number); out.err != nil {
		return core.MachineStop{}, out.err
	}

	return f.stopped, nil
}

// Compose, unlike every other method here, can succeed with a non-zero status: the fake answers
// both from the same outcome, the way the real Compose runner does.
func (f *fakeEngine) Compose(dir string, args []string) (int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Compose", Dir: dir, Command: args})

	out := f.outcomeFor(number)

	return out.status, out.err
}

// Logs, like Restart, only ever answers with an error: there is nothing to read back. Services is
// reused from Restart's own field — logs asks for the same shape, so there is no new one for it.
func (f *fakeEngine) Logs(dir string, services []string, options core.LogOptions) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Logs", Dir: dir, Services: services, LogOptions: options})

	return f.outcomeFor(number).err
}

// Anonymise, like Restart and Dump, only ever answers with an error: there is nothing to read
// back, so no new call field is needed beyond Dir, which already exists.
func (f *fakeEngine) Anonymise(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Anonymise", Dir: dir})

	return f.outcomeFor(number).err
}

// Capture, unlike every other method here except Compose, can succeed with a non-zero status
// alongside the string it read: the fake answers all three from the same outcome, the way the
// real Capture does.
func (f *fakeEngine) Capture(dir, service string, command []string) (string, int, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Capture", Dir: dir, Service: service, Command: command})

	out := f.outcomeFor(number)
	if f.byCommand != nil {
		if answer, ok := f.byCommand(command); ok {
			out = answer
		}
	}

	return out.captured, out.status, out.err
}

// XdebugHost, like Environments and Installed, asks nothing about a project: there is no
// directory to record. It answers from the same outcome.captured field Capture reads, because
// debug-on's readback needs an address here and a program's output there, at different points in
// the same ordered log, and outcomes is what already gives each call its own answer.
func (f *fakeEngine) XdebugHost() (string, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "XdebugHost"})

	out := f.outcomeFor(number)

	return out.captured, out.err
}

// Diagnose answers from its outcome: the diagnosis, or the error that stopped one. Only is
// recorded rather than acted on — choosing the check is app.Doctor's, and this seam's job is to
// prove the --only value arrives unchanged.
func (f *fakeEngine) Diagnose(dir, only string) (core.Diagnosis, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Diagnose", Dir: dir, Only: only})

	out := f.outcomeFor(number)
	if out.err != nil {
		return core.Diagnosis{}, out.err
	}

	return out.diagnosis, nil
}

// Survey answers from its outcome: the collection, or the error that stopped the survey.
func (f *fakeEngine) Survey() (core.Collection, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Survey"})

	out := f.outcomeFor(number)
	if out.err != nil {
		return core.Collection{}, out.err
	}

	return out.collection, nil
}

// Collect records the collection it was handed, which is the whole of what clean decides: the
// deleting itself is app.Collector's, and this seam's job is to prove what reaches it.
func (f *fakeEngine) Collect(collection core.Collection) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Collect", Collection: collection})

	return f.outcomeFor(number).err
}

// TestFakeEngineRecordsCaptureAndXdebugHost is Slice 2's own RED/GREEN — not merely the two
// compile assertions holding. debug-on calls Capture twice (php -v, then php -i) and needs a
// different answer each time; this proves the fake forwards each outcome's captured/status/err
// correctly for both new methods, in isolation from any handler, because debugOn does not exist
// yet.
func TestFakeEngineRecordsCaptureAndXdebugHost(t *testing.T) {
	t.Run("both answer their recorded outcome", func(t *testing.T) {
		fake := &fakeEngine{
			outcomes: []outcome{
				{captured: "PHP 8.1.27 (cli)\n    with Xdebug v3.1.5"},
				{captured: "172.17.0.1"},
			},
		}

		output, status, err := fake.Capture("/code/shop", phpService, []string{"php", "-v"})
		if err != nil {
			t.Fatalf("Capture(...) err = %v, want nil", err)
		}
		if status != 0 {
			t.Fatalf("Capture(...) status = %d, want 0", status)
		}
		if output != fake.outcomes[0].captured {
			t.Fatalf("Capture(...) output = %q, want %q", output, fake.outcomes[0].captured)
		}

		host, err := fake.XdebugHost()
		if err != nil {
			t.Fatalf("XdebugHost() err = %v, want nil", err)
		}
		if host != fake.outcomes[1].captured {
			t.Fatalf("XdebugHost() = %q, want %q", host, fake.outcomes[1].captured)
		}

		want := []call{
			{Method: "Capture", Dir: "/code/shop", Service: phpService, Command: []string{"php", "-v"}},
			{Method: "XdebugHost"},
		}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	t.Run("both forward their recorded error", func(t *testing.T) {
		captureErr := errors.New("docker: no")
		hostErr := errors.New("no docker0")

		fake := &fakeEngine{
			outcomes: []outcome{
				{status: 42, err: captureErr},
				{err: hostErr},
			},
		}

		_, status, err := fake.Capture("/code/shop", phpService, []string{"php", "-i"})
		if status != 42 || err != captureErr {
			t.Fatalf("Capture(...) = (_, %d, %v), want (_, 42, %v)", status, err, captureErr)
		}

		if _, err := fake.XdebugHost(); err != hostErr {
			t.Fatalf("XdebugHost() err = %v, want %v", err, hostErr)
		}
	})
}

// answering substitutes fake for newEngine for the rest of this test, and restores the real
// factory when it ends. Six pins go first, unconditionally, because while a call site is still
// unrouted — the RED half of a test written against this helper — it reaches the real engine()
// and everything underneath it:
//
//   - HM_STATE_DIR: the real engine() opens its registry there; unset, that is the developer's
//     own ~/.hm.
//   - HM_HOSTS_FILE: Hosts.file() falls back to the literal /etc/hosts (app/hosts.go:186), and
//     write() copies over it through `sudo cp` wired to the real terminal (app/hosts.go:173).
//     Pointed at a temporary file that does not exist, Set and Remove both fail at os.ReadFile
//     before anything is written.
//   - HM_NON_INTERACTIVE: choose() refuses immediately once this is set (select.go:39),
//     independently of any interactive bool, so a RED run never blocks on a question.
//   - DOCKER_HOST: an unrouted call into toolinfo's DockerVersion dials with no deadline of its
//     own; pointed at a socket that cannot exist, the dial fails fast instead of hanging.
//   - t.Chdir(t.TempDir()): app/hosts.go:49 writes properties.json under the resolved project
//     root, which without this pin is the directory the test binary happens to run in — the
//     package's own checkout.
func answering(t *testing.T) *fakeEngine {
	t.Helper()

	t.Setenv("HM_STATE_DIR", t.TempDir())
	t.Setenv("HM_HOSTS_FILE", filepath.Join(t.TempDir(), "hosts"))
	t.Setenv("HM_NON_INTERACTIVE", "1")
	t.Setenv("DOCKER_HOST", "unix:///nonexistent")
	t.Chdir(t.TempDir())

	fake := &fakeEngine{}

	original := newEngine
	newEngine = func(_, _ io.Writer, _ bool) commands { return fake }
	t.Cleanup(func() { newEngine = original })

	return fake
}

func (f *fakeEngine) Share(dir, service string, port int, confirm func(string) bool) (core.Share, error) {
	// The question is put, the way the use case puts it, and its answer is what is recorded
	answer := true
	if confirm != nil {
		answer = confirm(f.project.Name)
	}

	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Share", Dir: dir, Service: service, Port: port, Confirm: answer})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Share{}, out.err
	}

	if !answer {
		return core.Share{Project: f.project.Name, Declined: true}, nil
	}

	return f.share, nil
}

func (f *fakeEngine) Unshare(dir string) (bool, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Unshare", Dir: dir})

	if out := f.outcomeFor(number); out.err != nil {
		return false, out.err
	}

	return f.unshared, nil
}

// HoldShare returns at once: the tunnel it holds is closed as soon as it is opened.
func (f *fakeEngine) HoldShare(relay string) error {
	f.calls = append(f.calls, call{Method: "HoldShare", Key: relay})

	return f.outcomeFor(len(f.calls) - 1).err
}

func (f *fakeEngine) Tunnel(dir, service string, port int) (core.Tunnel, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Tunnel", Dir: dir, Service: service, Port: port})

	if out := f.outcomeFor(number); out.err != nil {
		return core.Tunnel{}, out.err
	}

	return f.tunnel, nil
}

func (f *fakeEngine) CloseTunnels(dir, service string) (bool, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CloseTunnels", Dir: dir, Service: service})

	if out := f.outcomeFor(number); out.err != nil {
		return false, out.err
	}

	return f.tunnelsClosed, nil
}

// HoldTunnel returns at once, like HoldShare.
func (f *fakeEngine) HoldTunnel(relay string) error {
	f.calls = append(f.calls, call{Method: "HoldTunnel", Key: relay})

	return f.outcomeFor(len(f.calls) - 1).err
}

func (f *fakeEngine) AIDoctor(dir string) (core.AIDoctorReport, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "AIDoctor", Dir: dir})

	return f.aiDoctor, f.outcomeFor(number).err
}

// AIInit, AIPull and AIReset run the real use case on the test's directories, because what they
// do is files: the fake only records the call and supplies the console.
func (f *fakeEngine) AIInit(dir string, options app.AIInitOptions, interactive bool, console app.AIConsole) int {
	f.calls = append(f.calls, call{Method: "AIInit", Dir: dir})

	return f.ai(dir, console).Init(options, interactive)
}

func (f *fakeEngine) AIPull(dir string, force bool, console app.AIConsole) int {
	f.calls = append(f.calls, call{Method: "AIPull", Dir: dir})

	return f.ai(dir, console).Pull(force)
}

func (f *fakeEngine) AIReset(dir string, confirm, interactive bool, console app.AIConsole) int {
	f.calls = append(f.calls, call{Method: "AIReset", Dir: dir})

	return f.ai(dir, console).Reset(confirm, interactive)
}

func (f *fakeEngine) ai(dir string, console app.AIConsole) *app.AITools {
	tools := *f.aiTools
	tools.Dir, tools.Console = dir, console

	return &tools
}

func (f *fakeEngine) Compatibility() (string, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Compatibility"})

	return f.compatibility, f.outcomeFor(number).err
}

func (f *fakeEngine) DockerReachable() bool {
	f.calls = append(f.calls, call{Method: "DockerReachable"})

	return !f.dockerDown
}

func (f *fakeEngine) Equivalents() (map[string]string, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Equivalents"})

	return f.equivalents, f.outcomeFor(number).err
}

func (f *fakeEngine) InstallSettings() ([]core.Setting, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "InstallSettings"})

	if f.settings == nil {
		f.settings = core.DefaultInstallSettings()
	}

	return append([]core.Setting(nil), f.settings...), f.outcomeFor(number).err
}

func (f *fakeEngine) SaveInstallSettings(settings []core.Setting) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "SaveInstallSettings"})

	f.settings = append([]core.Setting(nil), settings...)

	return f.outcomeFor(number).err
}

func (f *fakeEngine) CopyIntoService(dir, service, source, target string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "CopyIntoService", Dir: dir, Service: service,
		Paths: []string{source}, Path: target})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) ClearAnonymisation(dir string) error {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "ClearAnonymisation", Dir: dir})

	return f.outcomeFor(number).err
}

func (f *fakeEngine) Versions(running string) (core.Versions, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Versions"})
	f.versionsRunning = running

	return f.versions, f.outcomeFor(number).err
}

// Switch records the version it was asked for in Key, and everything it was asked in
// switchOptions.
func (f *fakeEngine) Switch(options core.SwitchOptions) (core.Switched, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Switch", Key: options.Version})
	f.switchOptions = options

	return f.switched, f.outcomeFor(number).err
}

func (f *fakeEngine) Verify(dir string, options core.VerifyOptions) (core.Verification, error) {
	number := len(f.calls)
	f.calls = append(f.calls, call{Method: "Verify", Dir: dir, VerifyOptions: options})

	return f.verification, f.outcomeFor(number).err
}
