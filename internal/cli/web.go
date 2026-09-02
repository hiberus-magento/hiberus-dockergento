package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/internal/api"
)

// The port the dashboard listens on by default. Configurable, because a machine is somebody's and
// there is no port nobody ever wants.
const defaultPort = 8420

// running is what is written down about a server that is up, so that a later `down` or `status`
// can find it. Beside the proxy's own state, for the same reason: it is a fact about this machine
// and not about any project.
type running struct {
	PID     int    `json:"pid"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	Started string `json:"started"`
}

func (r running) address() string {
	return fmt.Sprintf("http://127.0.0.1:%d/?token=%s", r.Port, r.Token)
}

// web is the browser interface, next to `tui` for the terminal one, and it behaves like the proxy
// on purpose: something you bring up once and forget about, not something that holds a terminal.
func web(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	action := "up"
	port := defaultPort
	foreground := false

	for at := 0; at < len(args); at++ {
		switch argument := args[at]; argument {
		case "up", "down", "status":
			action = argument
		case "--foreground", "-f":
			foreground = true
		case "--port", "-p":
			if at+1 >= len(args) {
				return failure(stderr, jsonOutput, "web", exitUsage, "missing_value",
					"--port needs a number", binaryName()+" web --port 8420")
			}

			at++

			number, err := strconv.Atoi(args[at])
			if err != nil {
				return failure(stderr, jsonOutput, "web", exitUsage, "invalid_argument",
					"--port needs a number", binaryName()+" web --port 8420")
			}

			port = number
		default:
			return failure(stderr, jsonOutput, "web", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument),
				binaryName()+" web [up|down|status] [--port 8420]")
		}
	}

	switch action {
	case "down":
		return webDown(stdout, stderr, jsonOutput)
	case "status":
		return webStatus(stdout, jsonOutput)
	}

	if foreground {
		return webHere(port, stdout, stderr, jsonOutput)
	}

	return webUp(port, stdout, stderr, jsonOutput)
}

// webHere runs the server in this process, which is what the detached one does once it has been
// started and what somebody debugging it wants.
func webHere(port int, stdout, stderr io.Writer, jsonOutput bool) int {
	token, err := api.NewToken()
	if err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "no_token", err.Error(), "")
	}

	// Loopback only, and not as a default somebody can widen: this API reads database
	// credentials and stops environments, and a machine on a shared network is the normal case
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return failure(stderr, jsonOutput, "web", exitBlocked, "port_taken",
			fmt.Sprintf("Port %d is not free: %s", port, err),
			fmt.Sprintf("%s web --port %d", binaryName(), port+1))
	}

	state := running{PID: os.Getpid(), Port: port, Token: token, Started: time.Now().Format("2006-01-02 15:04")}

	if err := writeState(state); err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "state_unwritable", err.Error(), "")
	}
	defer os.Remove(stateFile()) //nolint:errcheck

	server := api.Server{Engine: engine(stdout, stderr, jsonOutput), Token: token, Version: reportedVersion()}

	fmt.Fprintf(stdout, "%s\n", link(state.address()))

	if err := http.Serve(listener, server.Handler()); err != nil { //nolint:gosec
		return failure(stderr, jsonOutput, "web", exitError, "server_stopped", err.Error(), "")
	}

	return exitOK
}

// webUp starts the server in the background and waits until it answers.
//
// Waiting matters: a command that returns before the thing it started is reachable hands somebody
// a link that fails once and works on the second try, which is worse than being slow.
func webUp(port int, stdout, stderr io.Writer, jsonOutput bool) int {
	if state, ok := readState(); ok && alive(state.PID) {
		return served(state, "already running", stdout, jsonOutput)
	}

	executable, err := os.Executable()
	if err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "no_executable", err.Error(), "")
	}

	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "no_state_dir", err.Error(), "")
	}

	log, err := os.OpenFile(logFile(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "no_log", err.Error(), "")
	}
	defer log.Close()

	child := exec.Command(executable, "web", "--foreground", "--port", strconv.Itoa(port))
	child.Stdout = log
	child.Stderr = log
	child.Env = os.Environ()

	// Its own session, so closing the terminal that started it does not take it down
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := child.Start(); err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "not_started", err.Error(), "")
	}

	// Waited for in a goroutine: `Wait` blocks until the child exits, and the whole point is that
	// this one does not
	finished := make(chan error, 1)
	go func() { finished <- child.Wait() }()

	state, err := waitForServer(child, finished)
	if err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "not_started", err.Error(),
			"cat "+logFile())
	}

	return served(state, "started", stdout, jsonOutput)
}

// waitForServer waits until the server is actually reachable.
//
// Until it is reachable, and not merely started: a command that returns before the thing it
// launched answers hands somebody a link that fails once and works on the second try, which is
// worse than being slow.
func waitForServer(child *exec.Cmd, finished <-chan error) (running, error) {
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		if state, ok := readState(); ok && state.PID == child.Process.Pid && answers(state) {
			return state, nil
		}

		select {
		case <-finished:
			return running{}, fmt.Errorf("the server stopped straight away, see %s", logFile())
		case <-time.After(50 * time.Millisecond):
		}
	}

	return running{}, errors.New("the server did not answer in ten seconds")
}

func answers(state running) bool {
	client := http.Client{Timeout: time.Second}

	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health?token=%s", state.Port, state.Token))
	if err != nil {
		return false
	}
	defer response.Body.Close()

	return response.StatusCode == http.StatusOK
}

func webDown(stdout, stderr io.Writer, jsonOutput bool) int {
	state, ok := readState()
	if !ok || !alive(state.PID) {
		os.Remove(stateFile()) //nolint:errcheck

		if jsonOutput {
			return document(stdout, stderr, "web", map[string]any{"running": false, "stopped": false})
		}

		fmt.Fprintf(stdout, "%s\n", good("The web interface is not running."))

		return exitOK
	}

	if err := syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
		return failure(stderr, jsonOutput, "web", exitError, "not_stopped", err.Error(), "")
	}

	os.Remove(stateFile()) //nolint:errcheck

	if jsonOutput {
		return document(stdout, stderr, "web", map[string]any{"running": false, "stopped": true})
	}

	fmt.Fprintf(stdout, "%s\n", good("Stopped."))

	return exitOK
}

func webStatus(stdout io.Writer, jsonOutput bool) int {
	state, ok := readState()
	up := ok && alive(state.PID)

	if jsonOutput {
		answer := map[string]any{"running": up}
		if up {
			answer["port"] = state.Port
			answer["url"] = state.address()
			answer["pid"] = state.PID
			answer["started"] = state.Started
		}

		return document(stdout, nil, "web", answer)
	}

	if !up {
		fmt.Fprintf(stdout, "\n%s\n\n", "The web interface is not running.")
		fmt.Fprintf(stdout, "  %s\n\n", warning(binaryName()+" web"))

		return exitOK
	}

	fmt.Fprintf(stdout, "\n%s\n\n", header("The web interface is running"))
	fmt.Fprintf(stdout, "   %-10s %s\n", "since", state.Started)
	fmt.Fprintf(stdout, "   %-10s %s\n\n", "address", link(state.address()))

	return exitOK
}

// served prints where it is, which is the only thing anybody wants from this command.
func served(state running, what string, stdout io.Writer, jsonOutput bool) int {
	if jsonOutput {
		return document(stdout, nil, "web", map[string]any{
			"running": true, "port": state.Port, "url": state.address(), "pid": state.PID,
		})
	}

	fmt.Fprintf(stdout, "\n%s\n\n", good("The web interface is "+what+"."))
	fmt.Fprintf(stdout, "   %s\n\n", link(state.address()))
	fmt.Fprintf(stdout, "   %s\n\n", warning(binaryName()+" web down   # to stop it"))

	return exitOK
}

func document(stdout, stderr io.Writer, command string, data any) int {
	body, err := json.MarshalIndent(contract.Success(command, data), "", "  ")
	if err != nil {
		if stderr != nil {
			fmt.Fprintln(stderr, err)
		}

		return exitError
	}

	fmt.Fprintf(stdout, "%s\n", body)

	return exitOK
}

// stateDir is where this machine's own facts live, beside the proxy's: a running server is a fact
// about the machine and not about any project.
func stateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}

	return filepath.Join(home, ".hm")
}

func stateFile() string { return filepath.Join(stateDir(), "web.json") }
func logFile() string   { return filepath.Join(stateDir(), "web.log") }

func writeState(state running) error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}

	body, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Readable only by its owner: the token in it is what stands between a page in a browser and
	// this machine's environments
	return os.WriteFile(stateFile(), body, 0o600)
}

func readState() (running, bool) {
	body, err := os.ReadFile(stateFile())
	if err != nil {
		return running{}, false
	}

	var state running
	if err := json.Unmarshal(body, &state); err != nil {
		return running{}, false
	}

	return state, state.PID > 0
}

// alive reports whether that process is still there. Signal zero asks the kernel without sending
// anything, which is how a stale state file is told from a running server.
func alive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}

func reportedVersion() string {
	if Version != "" {
		return Version
	}

	return "dev"
}
