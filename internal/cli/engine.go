package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// commands is what a command asks of the tool. Narrow on purpose: it names what the handlers
// routed through newEngine call and nothing else, so a test can answer for the engine without a
// daemon.
type commands interface {
	Resolve(dir string) (core.Project, error)
	Environments() ([]core.Environment, error)
	Describe(dir string, withSecrets bool) (core.Description, error)
	Exec(dir, service string, command []string, options core.ExecOptions) (int, error)
	RestartWith(dir string, options core.RestartOptions) error
	AfterStart(dir string) error
	InstallCertificate(dir, domain string) error
	Rebuild(dir string, services []string) error
	StopWith(dir string, options core.StopOptions) error
	Start(dir string, options dockergento.StartOptions) error
	Down(dir string, options core.DownOptions, interactive bool) (string, error)
	Query(dir, statement string, out io.Writer) (int, error)
	Import(dir string, options app.ImportOptions) error
	Feed(dir string) (int, error)
	Console(dir string) (int, error)
	Setup(dir string, options core.SetupOptions, interactive bool) error
	ConfigureForDevelopment(dir string) error
	Update(options core.UpdateOptions) (core.Updated, error)
	AgentPermissions(strict bool) (core.AgentPermissions, error)
	WriteAgentContext(dir string) (core.AgentContextResult, error)
	CreateProject(dir string, options core.CreateProjectOptions, interactive bool) error
	CopyInto(dir string, paths []string, all bool) error
	CopyFrom(dir string, paths []string) error
	CopyIntoService(dir, service, source, target string) error
	ClearAnonymisation(dir string) error
	DefaultMagentoFiles() ([]string, error)
	ClearVendor(dir string) error
	Property(project core.Project, key string) string
	Dump(dir, path string) error
	Installed() (core.Installation, core.Tooling)
	SetHost(dir, domain string, database bool) error
	RemoveHost(domain string) error
	StopEverything(dir string, interactive bool) (core.MachineStop, error)
	Compose(dir string, args []string) (int, error)
	Logs(dir string, services []string, options core.LogOptions) error
	Anonymise(dir string) error
	Capture(dir, service string, command []string) (string, int, error)
	XdebugHost() (string, error)
	Diagnose(dir, only string) (core.Diagnosis, error)
	Survey() (core.Collection, error)
	Collect(collection core.Collection) error
	Share(dir, service string, port int, confirm func(string) bool) (core.Share, error)
	Unshare(dir string) (bool, error)
	HoldShare(relay string) error
	Tunnel(dir, service string, port int) (core.Tunnel, error)
	CloseTunnels(dir, service string) (bool, error)
	HoldTunnel(relay string) error
	Compatibility() (string, error)
	DockerReachable() bool
	Equivalents() (map[string]string, error)
	InstallSettings() ([]core.Setting, error)
	SaveInstallSettings(settings []core.Setting) error
	Versions(running string) (core.Versions, error)
	Switch(options core.SwitchOptions) (core.Switched, error)
	Verify(dir string, options core.VerifyOptions) (core.Verification, error)
	AIDoctor(dir string) (core.AIDoctorReport, error)
	AIInit(dir string, options app.AIInitOptions, interactive bool, console app.AIConsole) int
	AIPull(dir string, force bool, console app.AIConsole) int
	AIReset(dir string, confirm, interactive bool, console app.AIConsole) int
	Status(dir string) (core.Status, error)
}

var _ commands = (*dockergento.Engine)(nil)

// newEngine is the seam a test substitutes: production never sets it, so every command still
// gets the same *dockergento.Engine engine() has always built.
var newEngine = func(stdout, stderr io.Writer, jsonOutput bool) commands {
	return engine(stdout, stderr, jsonOutput)
}

// engine builds the tool for this invocation.
//
// Everything the command line adds is here and nowhere else: the name it was invoked as, which
// goes into every hint, and where the steps that take a while announce themselves. The engine
// itself has no opinion about terminals — which is what lets the same calls answer an HTTP request
// later without any of this coming along.
func engine(stdout, stderr io.Writer, jsonOutput bool) *dockergento.Engine {
	announce, progress := reporting(stdout, stderr, jsonOutput)

	return dockergento.New(dockergento.Options{
		Binary:   binaryName(),
		Version:  Version,
		Assets:   toolAssets(),
		StateDir: os.Getenv("HM_STATE_DIR"),

		// --force is a decision about one invocation, which is why it is read from the
		// environment the flag set and never from a file
		Forced: os.Getenv("HM_FORCE") == "1",

		// How the steps that take a while are shown: see reporting
		Announce: announce,
		Progress: progress,

		// The question lives here for the same reason: the engine has no terminal and should not
		// learn about one
		Ask:    ask,
		Choose: choose,

		// The line under each "Copying" goes to stderr, and only to a terminal
		Copying: copyWatch(stderr, jsonOutput),
	})
}

// projectOr resolves the project this directory belongs to, or reports why it could not.
func projectOr(stderr io.Writer, jsonOutput bool, command string) (core.Project, int) {
	directory, err := os.Getwd()
	if err != nil {
		return core.Project{}, failure(stderr, jsonOutput, command, exitError, "no_working_directory",
			err.Error(), "")
	}

	project, err := newEngine(nil, nil, jsonOutput).Resolve(directory)

	// A refusal already says why and what to do, which the generic answer below would hide
	var refusal core.Refusal
	if asRefusal(err, &refusal) {
		return core.Project{}, report(stderr, jsonOutput, command, err)
	}

	if err != nil || project.Name == "" {
		return core.Project{}, report(stderr, jsonOutput, command, app.NotConfigured("", binaryName()))
	}

	return project, 0
}

// codeProjectOr is projectOr for the commands that work on the code, and it says whose code that
// is when it is not the code in front of the person: from a worktree with no environment of its
// own, the containers are the main checkout's.
//
// Ported from bin/run's validate_worktree (:261-262), for the names hm_operates_on_code lists
// (console/helpers/worktree.sh:99-108) that this binary answers: magento, composer, npm,
// n98-magerun, test-unit, test-integration, exec and bash, which reaches the container through
// exec. The same text, on stderr like the shell's print_warning_line, and under --json too, as
// there: stdout is the container's, and a pipe reading it must not find a note in it.
func codeProjectOr(stderr io.Writer, jsonOutput bool, command string) (core.Project, int) {
	project, code := projectOr(stderr, jsonOutput, command)

	if code == 0 && project.ServesAnotherCheckout() {
		fmt.Fprint(stderr, warning(fmt.Sprintf(
			"Note: the containers serve the code of %s, not this worktree.", project.Root))+"\n")
	}

	return project, code
}

// here is the directory the command was run in, which is what every question is asked about.
func here() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}

	return directory
}

// binaryName is what the tool was invoked as, because every action printed is a command the
// reader is meant to be able to paste.
func binaryName() string {
	if name := os.Getenv("COMMAND_BIN_NAME"); name != "" {
		return name
	}

	return "hm"
}
