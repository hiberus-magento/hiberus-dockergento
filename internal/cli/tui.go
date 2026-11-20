package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

//
// The fleet dashboard, ported from console/commands/tui.sh.
//
// It presents; the CLI decides. Data comes from this binary's own `list`, `doctor` and `describe`
// in JSON, and actions are its own commands run in the environment's directory, so the
// protections —worktree guardrails, exit codes— are inherited rather than bypassed.
//
// Three stages, as in the shell: loading talks to the binary, composing turns its JSON into lines
// already cut to the width, and painting turns lines into one write. Only the first two cost
// anything, and they run when the data or the terminal width changes.
//
// Everything that needs a terminal is a field: the keys are a reader, the size a function, the
// wait for "any key" a function, and the commands go through a runner — which is what lets the
// whole dashboard run in a test with no terminal and no Docker.
//

// dashRunner runs the binary.
type dashRunner interface {
	// Capture runs it in dir (the current directory when empty) and returns what it printed on
	// stdout, and on stderr too when combined, and whether it succeeded.
	Capture(dir string, combined bool, args ...string) (string, bool)
	// Hand gives it the terminal and returns its exit code.
	Hand(dir string, args ...string) int
}

// binaryRunner runs this same binary, which is what the shell's "$COMMAND_BIN_DIR/bin/hm" was.
type binaryRunner struct{ path string }

func (b binaryRunner) Capture(dir string, combined bool, args ...string) (string, bool) {
	command := exec.Command(b.path, args...)
	command.Dir = dir

	output := &bytes.Buffer{}
	command.Stdout = output

	if combined {
		command.Stderr = output
	}

	err := command.Run()

	// What a command substitution keeps: trailing newlines go
	return strings.TrimRight(output.String(), "\n"), err == nil
}

func (b binaryRunner) Hand(dir string, args ...string) int {
	command := exec.Command(b.path, args...)
	command.Dir = dir
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr

	err := command.Run()

	var exited *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exited):
		return exited.ExitCode()
	}

	return 127
}

const (
	dashLoading = "loading…"
	dashGone    = "that environment's directory is gone"
)

type dashboard struct {
	out    io.Writer
	keys   *bufio.Reader
	runner dashRunner

	size  func() (rows, cols int)
	now   func() string
	pause func()
	raw   func(on bool)

	mu        sync.Mutex
	suspended bool

	// handing is whether a command has the terminal; read by the interruption handler, which
	// cannot take mu because the action holds it for as long as the command runs
	handing  atomic.Bool
	inScreen bool
	hidden   bool

	rows, cols   int
	composedCols int

	view         string
	selected     int
	firstVisible int
	detailOffset int
	message      string
	loadedAt     string

	fleetJSON, doctorJSON, detailJSON string
	detailName                        string
	environments                      []map[string]any

	fleetHeader string
	fleetRows   []string
	doctorRows  []string
	detailRows  []string
}

func newDashboard(out io.Writer, keys io.Reader, runner dashRunner) *dashboard {
	return &dashboard{
		out: out, keys: bufio.NewReader(keys), runner: runner,
		size:  func() (int, int) { return 24, 80 },
		now:   func() string { return time.Now().Format("15:04:05") },
		pause: func() {},
		view:  "fleet", message: dashLoading, rows: 24, cols: 80,
	}
}

// tui is the command: the dashboard on this terminal, or a refusal when there is none.
func tui(stdout, stderr io.Writer, jsonOutput bool) int {
	// Drawing on something that is not a terminal would write control codes into a file
	if !isTerminal(stdout) {
		return failure(stderr, jsonOutput, "tui", exitUsage, "requires_terminal",
			"The dashboard needs a terminal, and this output is not one", binaryName()+" list --json")
	}

	terminal, _ := stdout.(*os.File)

	self, err := os.Executable()
	if err != nil {
		return failure(stderr, jsonOutput, "tui", exitError, "no_binary", err.Error(), "")
	}

	board := newDashboard(terminal, os.Stdin, binaryRunner{path: self})
	board.size = func() (int, int) { return terminalSize(terminal) }
	board.pause = anyKey
	board.raw = rawInput()

	// Redraw on resize without waiting for a key; and whatever ends the process, the terminal is
	// given back first
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)

	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(resized)
	defer signal.Stop(interrupted)
	defer board.restore()

	go func() {
		for range resized {
			board.resize()
		}
	}()

	go board.watchInterrupts(interrupted)

	return board.run()
}

// watchInterrupts gives the terminal back and ends the process on an interruption — except a
// Ctrl-C while a command has the terminal. The terminal sends that to the whole foreground group:
// it is the command's to end (logs -f is left that way), and the dashboard comes back once it
// has, as the shell's did. A TERM is meant for the dashboard, handed command or not.
func (d *dashboard) watchInterrupts(signals <-chan os.Signal) {
	for received := range signals {
		if received == os.Interrupt && d.handing.Load() {
			continue
		}

		d.restore()
		exitProcess(130)

		return
	}
}

// terminalSize is the terminal's size, always usable: the terminal itself, then the shell's
// variables, then the size every terminal has assumed since 1978.
func terminalSize(terminal *os.File) (int, int) {
	if terminal != nil {
		if cols, rows, err := term.GetSize(int(terminal.Fd())); err == nil && rows > 0 && cols > 0 {
			return rows, cols
		}
	}

	rows, errRows := strconv.Atoi(os.Getenv("LINES"))
	cols, errCols := strconv.Atoi(os.Getenv("COLUMNS"))

	if errRows == nil && errCols == nil && rows > 0 && cols > 0 {
		return rows, cols
	}

	return 24, 80
}

// rawInput switches stdin between raw and cooked, when it is a terminal: keys arrive one at a time
// and unechoed while the dashboard reads them, and normally while a command has the terminal.
func rawInput() func(bool) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}

	var saved *term.State

	return func(on bool) {
		if on && saved == nil {
			saved, _ = term.MakeRaw(fd)
		} else if !on && saved != nil {
			term.Restore(fd, saved) //nolint:errcheck
			saved = nil
		}
	}
}

// anyKey waits for one key on the terminal itself, as the shell did with `read -rsn1 </dev/tty`.
func anyKey() {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return
	}
	defer tty.Close()

	if state, err := term.MakeRaw(int(tty.Fd())); err == nil {
		defer term.Restore(int(tty.Fd()), state) //nolint:errcheck
	}

	tty.Read(make([]byte, 1)) //nolint:errcheck
}

// ------------------------------------------------------------------ the loop

// run is the dashboard from the first frame to giving the terminal back.
func (d *dashboard) run() int {
	d.mu.Lock()
	d.enterScreen()
	d.hideCursor()
	d.setRaw(true)
	d.updateSize()
	d.loadFleet()
	d.mu.Unlock()

	for {
		d.mu.Lock()
		d.paint()
		d.mu.Unlock()

		key, err := dashKey(d.keys)
		if err != nil {
			key = "q"
		}

		d.mu.Lock()
		quit := d.handle(key)
		d.mu.Unlock()

		if quit {
			break
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.showCursor()
	d.leaveScreen()
	d.setRaw(false)

	return exitOK
}

// handle is one key. It reports whether the dashboard is done.
func (d *dashboard) handle(key string) bool {
	count := len(d.fleetRows)

	switch key {
	case "q", "ctrl-c":
		return true
	case "up", "k":
		if d.view == "detail" {
			if d.detailOffset > 0 {
				d.detailOffset--
			}
		} else if d.selected > 0 {
			d.selected--
		}
	case "down", "j":
		if d.view == "detail" {
			d.detailOffset++
		} else if d.selected < count-1 {
			d.selected++
		}
	case "enter":
		if d.view == "fleet" && count > 0 && d.loadDetail() {
			d.view = "detail"
		}
	case "esc", "h", "backspace":
		d.view = "fleet"
	case "g":
		d.loadFleet()
	case "s", "x", "r", "l", "o":
		if count == 0 {
			break
		}

		switch key {
		case "s":
			d.runAction("start")
		case "x":
			d.runAction("stop")
		case "r":
			d.runAction("restart")
		case "l":
			d.runAction("logs", "-f", "--tail", "100")
		case "o":
			d.openInBrowser()
		}
	case "?":
		d.showKeys()
	}

	return false
}

// resize is what a window resize does: the new size, and a frame drawn to it straight away —
// unless a command has the terminal, whose output a frame would paint over.
func (d *dashboard) resize() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.inScreen || d.suspended {
		return
	}

	d.updateSize()
	d.paint()
}

// ------------------------------------------------------------------ data

func (d *dashboard) loadFleet() {
	d.message = dashLoading
	d.paint()

	d.fleetJSON = d.capture("", "list", "--json")
	d.doctorJSON = d.capture("", "doctor", "--json")
	d.loadedAt = d.now()
	d.message = ""

	d.environments = dashEnvironments(d.fleetJSON)
	count := len(d.environments)

	if count == 0 {
		d.selected = 0
	} else if d.selected >= count {
		d.selected = count - 1
	}

	d.compose()
}

// capture is `$("$HM" ... 2>/dev/null || echo '{}')`: a failure still has what it printed read,
// followed by an empty document.
func (d *dashboard) capture(dir string, args ...string) string {
	output, ok := d.runner.Capture(dir, false, args...)
	if ok {
		return output
	}

	if output == "" {
		return "{}"
	}

	return output + "\n{}"
}

func (d *dashboard) loadDetail() bool {
	root := d.field("root")

	if !isDirectory(root) {
		d.detailJSON = ""
		d.message = dashGone

		return false
	}

	d.message = dashLoading
	d.paint()
	d.detailJSON = d.capture(root, "describe", "--json")
	d.message = ""

	// The name is captured with the data: moving through the list behind an open detail must not
	// retitle it with another environment's name
	d.detailName = d.field("name")
	d.detailOffset = 0
	d.detailRows = dashDetailLines(d.detailJSON, d.cols-4)

	return true
}

// field is a field of the selected environment, empty when there is none.
func (d *dashboard) field(name string) string {
	if d.selected < 0 || d.selected >= len(d.environments) {
		return ""
	}

	return dashJoined(d.environments[d.selected][name])
}

func isDirectory(path string) bool {
	if path == "" {
		return false
	}

	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// ------------------------------------------------------------------ actions

// runAction hands the terminal over, runs the command where the environment lives, and comes back.
// Running it there is what makes the CLI's own protections apply, and letting it write normally
// means a failure shows its whole error instead of a cropped one inside a box.
func (d *dashboard) runAction(args ...string) {
	root, name := d.field("root"), d.field("name")

	if !isDirectory(root) {
		d.message = dashGone

		return
	}

	d.suspend()

	fmt.Fprintf(d.out, "%s%s %s%s  in %s\n\n", code("\033[1m"), binaryName(), strings.Join(args, " "),
		code(reset), name)

	d.handing.Store(true)
	status := d.runner.Hand(root, args...)
	d.handing.Store(false)

	fmt.Fprint(d.out, "\n")

	if status == 0 {
		fmt.Fprint(d.out, good("Done. Press any key to go back to the dashboard.\n"))
	} else {
		fmt.Fprint(d.out, bad(fmt.Sprintf("Failed with exit code %d. Press any key to go back to the dashboard.\n", status)))
	}

	d.pause()
	d.resume()
	d.loadFleet()
}

// openInBrowser asks `launch` for the storefront: resolving the address and saying when there is
// none is its work, not the dashboard's.
func (d *dashboard) openInBrowser() {
	root := d.field("root")

	if !isDirectory(root) {
		d.message = dashGone

		return
	}

	output, ok := d.runner.Capture(root, true, "launch", "--json")
	documents := dashDocuments(output)

	var document any
	if len(documents) > 0 {
		document = documents[0]
	}

	if ok {
		d.message = "opened " + dashOr(dashAt(document, "data", "url"), "")
	} else if len(documents) > 0 {
		d.message = dashOr(dashAt(document, "error", "message"), "could not open it")
	} else {
		d.message = ""
	}
}

func (d *dashboard) showKeys() {
	d.suspend()

	fmt.Fprintf(d.out, "%sKeys%s\n\n", code("\033[1m"), code(reset))

	for _, key := range [][2]string{
		{"up/down, k/j", "move through the list"},
		{"enter", "open the selected environment"},
		{"esc", "back to the fleet"},
		{"s", "start the environment"},
		{"x", "stop it"},
		{"r", "restart it"},
		{"l", "follow its logs"},
		{"o", "open it in the browser"},
		{"g", "refresh the data"},
		{"q", "quit"},
	} {
		fmt.Fprintf(d.out, "  %-12s %s\n", key[0], key[1])
	}

	fmt.Fprint(d.out, "\nPress any key to go back.\n")

	d.pause()
	d.resume()
}

// ------------------------------------------------------------------ the terminal

func (d *dashboard) updateSize() {
	d.rows, d.cols = d.size()
}

func (d *dashboard) setRaw(on bool) {
	if d.raw != nil {
		d.raw(on)
	}
}

// enterScreen takes a screen of its own; leaving it gives the user back exactly what they had,
// scrollback included.
func (d *dashboard) enterScreen() {
	if d.inScreen {
		return
	}

	fmt.Fprint(d.out, "\033[?1049h")
	d.inScreen = true
	d.updateSize()
}

func (d *dashboard) leaveScreen() {
	if !d.inScreen {
		return
	}

	fmt.Fprint(d.out, "\033[?1049l")
	d.inScreen = false
}

func (d *dashboard) hideCursor() {
	fmt.Fprint(d.out, "\033[?25l")
	d.hidden = true
}

func (d *dashboard) showCursor() {
	fmt.Fprint(d.out, "\033[?25h")
	d.hidden = false
}

// suspend hands the terminal over to another command, and resume takes it back.
func (d *dashboard) suspend() {
	d.showCursor()
	d.leaveScreen()
	d.setRaw(false)
	d.suspended = true
}

func (d *dashboard) resume() {
	d.suspended = false
	d.enterScreen()
	d.hideCursor()
	d.setRaw(true)
}

// restore puts the terminal back whatever happened: main screen, cursor visible, input cooked.
// Idempotent, because it can arrive twice.
func (d *dashboard) restore() {
	if d.inScreen {
		fmt.Fprint(d.out, "\033[?1049l")
		d.inScreen = false
	}

	if d.hidden {
		fmt.Fprint(d.out, "\033[?25h")
		d.hidden = false
	}

	d.setRaw(false)
}

// ------------------------------------------------------------------ keys

// dashKey reads one keypress and names it. An escape sequence is the escape and the two bytes
// already waiting after it; an escape alone is Esc.
func dashKey(reader *bufio.Reader) (string, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	if first == 27 {
		if reader.Buffered() < 2 {
			return "esc", nil
		}

		rest := make([]byte, 2)
		if _, err := io.ReadFull(reader, rest); err != nil {
			return "esc", nil
		}

		return dashKeyName("\033" + string(rest)), nil
	}

	return dashKeyName(string(first)), nil
}

func dashKeyName(sequence string) string {
	switch sequence {
	case "\033[A", "\033OA":
		return "up"
	case "\033[B", "\033OB":
		return "down"
	case "\033[C", "\033OC":
		return "right"
	case "\033[D", "\033OD":
		return "left"
	case "\033":
		return "esc"
	case "\n", "\r", "":
		return "enter"
	case "\003":
		return "ctrl-c"
	case "\177", "\010":
		return "backspace"
	case "\t":
		return "tab"
	}

	return sequence
}
