package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

//
// The fleet dashboard. It presents; the CLI decides: data comes from the binary's own list, doctor
// and describe in JSON, and actions are the binary's own commands run in the environment's
// directory. Nothing here needs a terminal or Docker: keys come from a reader, frames are strings,
// and the commands it would run are recorded by a fake runner.
//

// ------------------------------------------------------------------ the fake runner

type dashCall struct {
	dir      string
	args     string
	combined bool
	handed   bool
}

type fakeDashRunner struct {
	calls   []dashCall
	answers map[string]string
	fails   map[string]bool
	handed  int
}

func (f *fakeDashRunner) Capture(dir string, combined bool, args ...string) (string, bool) {
	joined := strings.Join(args, " ")
	f.calls = append(f.calls, dashCall{dir: dir, args: joined, combined: combined})

	return f.answers[joined], !f.fails[joined]
}

func (f *fakeDashRunner) Hand(dir string, args ...string) int {
	f.calls = append(f.calls, dashCall{dir: dir, args: strings.Join(args, " "), handed: true})

	return f.handed
}

func (f *fakeDashRunner) argv() []string {
	all := []string{}
	for _, call := range f.calls {
		all = append(all, call.args)
	}

	return all
}

func fleetPayload(t *testing.T, roots ...string) string {
	t.Helper()

	environments := []map[string]any{}
	for at, root := range roots {
		environments = append(environments, map[string]any{
			"name": []string{"shop", "blog", "store", "outlet", "cms"}[at%5], "status": "running",
			"containers": map[string]int{"running": 3, "total": 4}, "branch": "main",
			"root": root, "worktree": "", "orphan": false, "has_metadata": true,
		})
	}

	payload, err := json.Marshal(map[string]any{"ok": true, "data": map[string]any{"environments": environments}})
	if err != nil {
		t.Fatal(err)
	}

	return string(payload)
}

const doctorPayload = `{"ok":true,"data":{"checks":[
 {"id":"docker","severity":"ok","message":"fine"},
 {"id":"disk","severity":"warning","message":"almost full"},
 {"id":"ports","severity":"error","message":"80 is taken"}]}}`

const describePayload = `{"ok":true,"data":{
 "project":{"name":"shop","root":"/code/shop","domain":"shop.test","status":"running",
  "urls":{"base":"https://shop.test/","admin":"https://shop.test/admin","mail":"","mailhog":"","rabbitmq":"http://127.0.0.1:15672","search":""}},
 "magento":{"version":"2.4.7","mode":"developer"},
 "services":[{"name":"phpfpm","state":"running","image":"hiberusmagento/php:8.3"},{"name":"db","state":"exited","image":"mariadb:10.6"}]}}`

type dashLab struct {
	board  *dashboard
	runner *fakeDashRunner
	out    *bytes.Buffer
	roots  []string
	pauses int
}

// lab is a dashboard on a 24x80 screen with as many environments as given, each with a directory
// that exists.
func lab(t *testing.T, keys string, environments int) *dashLab {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COMMAND_BIN_NAME", "hm")

	roots := []string{}
	for range environments {
		roots = append(roots, t.TempDir())
	}

	runner := &fakeDashRunner{
		answers: map[string]string{
			"list --json": fleetPayload(t, roots...), "doctor --json": doctorPayload,
			"describe --json": describePayload,
		},
		fails: map[string]bool{},
	}

	out := &bytes.Buffer{}
	result := &dashLab{runner: runner, out: out, roots: roots}

	board := newDashboard(out, strings.NewReader(keys), runner)
	board.size = func() (int, int) { return 24, 80 }
	board.now = func() string { return "10:11:12" }
	board.pause = func() { result.pauses++ }
	result.board = board

	return result
}

// lastFrame is the screen as the last frame left it, one string per row.
func (l *dashLab) lastFrame() []string {
	return l.board.frame()
}

func (l *dashLab) screen() string {
	return strings.Join(l.lastFrame(), "\n")
}

// ------------------------------------------------------------------ without a terminal

func TestTuiRefusesWhenTheOutputIsNotATerminal(t *testing.T) {
	answering(t)

	for _, args := range [][]string{
		{"tui"}, {"tui", "--json"}, {"--json", "tui"}, {"tui", "--foo"}, {"tui", "extra"}, {"tui", "--yes"},
	} {
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

		code := Run(args, stdout, stderr)

		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}

		if stdout.Len() != 0 {
			t.Errorf("%v: wrote to stdout: %q", args, stdout.String())
		}

		var document struct {
			Command string `json:"command"`
			Error   struct {
				Code    int    `json:"code"`
				Type    string `json:"type"`
				Message string `json:"message"`
				Hint    string `json:"hint"`
			} `json:"error"`
		}

		if err := json.Unmarshal(stderr.Bytes(), &document); err != nil {
			t.Fatalf("%v: stderr is not JSON: %q", args, stderr.String())
		}

		if document.Command != "tui" || document.Error.Type != "requires_terminal" ||
			document.Error.Code != 2 ||
			document.Error.Message != "The dashboard needs a terminal, and this output is not one" ||
			document.Error.Hint != "hm list --json" {
			t.Errorf("%v: %+v", args, document)
		}
	}
}

func TestTuiRefusesInTextWhenTextIsAskedFor(t *testing.T) {
	answering(t)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	code := Run([]string{"tui", "--no-json"}, stdout, stderr)

	if code != exitUsage || !strings.Contains(stderr.String(), "The dashboard needs a terminal") ||
		!strings.Contains(stderr.String(), "hm list --json") || stdout.Len() != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

// ------------------------------------------------------------------ rendering

func TestDashCutMarksWhatItCuts(t *testing.T) {
	cases := []struct {
		text  string
		width int
		want  string
	}{
		{"short", 10, "short"}, {"exactly10!", 10, "exactly10!"}, {"much too long", 8, "much to~"},
		{"abc", 1, "a"}, {"abc", 0, ""},
	}

	for _, c := range cases {
		if got := dashCut(c.text, c.width); got != c.want {
			t.Errorf("dashCut(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
		}
	}
}

func TestDashCutPathKeepsTheEnd(t *testing.T) {
	cases := []struct {
		path  string
		width int
		want  string
	}{
		{"/a/b", 10, "/a/b"}, {"/Users/me/projects/shop", 10, "~ects/shop"}, {"/abc", 1, "c"},
	}

	for _, c := range cases {
		if got := dashCutPath(c.path, c.width); got != c.want {
			t.Errorf("dashCutPath(%q, %d) = %q, want %q", c.path, c.width, got, c.want)
		}
	}
}

func TestDashColumnsGiveWayPathFirst(t *testing.T) {
	cases := []struct {
		width                                int
		name, status, services, branch, path int
	}{
		{120, 22, 9, 5, 18, 60}, {76, 22, 9, 5, 18, 16}, {70, 22, 9, 5, 29, 0}, {45, 22, 9, 5, 0, 0},
	}

	for _, c := range cases {
		name, status, services, branch, path := dashFleetColumns(c.width)
		if name != c.name || status != c.status || services != c.services || branch != c.branch || path != c.path {
			t.Errorf("width %d: %d %d %d %d %d", c.width, name, status, services, branch, path)
		}
	}
}

func TestDashHeaderIsAlignedWithTheRows(t *testing.T) {
	want := "PROJECT                STATUS    UP    BRANCH             PATH"
	if got := dashFleetHeader(76); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	if got := dashFleetHeader(45); got != "PROJECT                STATUS    UP   " {
		t.Errorf("narrow header %q", got)
	}
}

func TestDashRowsCarryFlagsPastTheCut(t *testing.T) {
	payload := `{"data":{"environments":[
	 {"name":"a-very-long-project-name-indeed","status":"running","containers":{"running":2,"total":5},
	  "branch":"feature/a-very-long-branch-name","root":"/Users/me/projects/a-very-long-project","worktree":"wt","orphan":true},
	 {"name":"","status":"stopped"},
	 {"name":"shop","status":"stopped","containers":{"running":0,"total":4},"branch":"","root":"/s","worktree":"","orphan":false}]}}`

	rows := dashFleetRows(payload, 76)

	want := []string{
		"a-very-long-pr~ [wt] ! running   2/5   feature/a-very-lo~ ~ry-long-project",
		"shop                   stopped   0/4   -                  /s",
	}

	if strings.Join(rows, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}

func TestDashRowsAcceptTheBarePayload(t *testing.T) {
	rows := dashFleetRows(`{"environments":[{"name":"shop","status":"running","containers":{"running":1,"total":1}}]}`, 45)

	if len(rows) != 1 || rows[0] != "shop                   running   1/1  " {
		t.Errorf("%q", rows)
	}

	if rows := dashFleetRows("not json", 80); len(rows) != 0 {
		t.Errorf("garbage gave rows: %q", rows)
	}
}

func TestDashDoctorShowsProblemsMostSevereFirst(t *testing.T) {
	lines := dashDoctorLines(doctorPayload, 78)

	want := []string{"ERROR  ports  80 is taken", "WARNING  disk  almost full"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Errorf("%q", lines)
	}

	if lines := dashDoctorLines(doctorPayload, 10); lines[0] != "ERROR  po~" {
		t.Errorf("not cut: %q", lines)
	}
}

// A failing doctor still has its document read: the shell appended `{}` to whatever it printed.
func TestDashDoctorReadsTheDocumentBeforeAFallback(t *testing.T) {
	lines := dashDoctorLines(doctorPayload+"\n{}", 78)

	if len(lines) != 2 {
		t.Errorf("%q", lines)
	}
}

func TestDashDetailDescribesOneEnvironment(t *testing.T) {
	want := []string{
		"Project    shop",
		"Status     running",
		"Domain     shop.test",
		"Magento    2.4.7  (developer)",
		"Root       /code/shop",
		"",
		"URLs",
		"  base      https://shop.test/",
		"  admin     https://shop.test/admin",
		"  rabbitmq  http://127.0.0.1:15672",
		"",
		"Services",
		"  phpfpm       running   hiberusmagento/php:8.3",
		"  db           exited    mariadb:10.6",
	}

	got := dashDetailLines(describePayload, 76)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDashDetailFillsTheBlanks(t *testing.T) {
	got := dashDetailLines(`{}`, 76)

	want := []string{"Project    ", "Status     ", "Domain     -", "Magento    unknown", "Root       ", "", "URLs", "", "Services"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%q", got)
	}
}

// ------------------------------------------------------------------ the frame

func TestTheFirstFrameSaysItIsReading(t *testing.T) {
	l := lab(t, "q", 2)

	l.board.run()

	if !strings.Contains(l.out.String(), "Reading the environments on this machine…") {
		t.Error("no reading frame")
	}

	if !strings.Contains(l.out.String(), "loading…") {
		t.Error("no loading message")
	}
}

func TestTheFleetFrame(t *testing.T) {
	l := lab(t, "q", 2)
	l.board.run()

	frame := l.lastFrame()

	if len(frame) != 24 {
		t.Fatalf("%d rows", len(frame))
	}

	checks := map[int]string{
		0: "Dockergento — environments on this machine", 1: "",
		2: "  ERROR  ports  80 is taken", 3: "  WARNING  disk  almost full", 4: "",
		5:  "  PROJECT                STATUS    UP    BRANCH             PATH",
		22: "data from 10:11:12",
		23: "j/k move   enter open   s/x/r start/stop/restart   ? keys   q quit",
	}

	for row, want := range checks {
		if frame[row] != want {
			t.Errorf("row %d = %q, want %q", row, frame[row], want)
		}
	}

	if !strings.HasPrefix(frame[6], "> shop ") || !strings.HasPrefix(frame[7], "  blog ") {
		t.Errorf("selection: %q / %q", frame[6], frame[7])
	}
}

func TestTheFooterIsLongWhenItFits(t *testing.T) {
	l := lab(t, "q", 1)
	l.board.size = func() (int, int) { return 24, 120 }
	l.board.run()

	if got := l.lastFrame()[23]; got != "j/k move   enter open   s start   x stop   r restart   l logs   o browser   g refresh   ? keys   q quit" {
		t.Errorf("%q", got)
	}
}

func TestAnEmptyFleetSaysHowToCreateOne(t *testing.T) {
	l := lab(t, "q", 0)
	l.board.run()

	screen := l.screen()
	if !strings.Contains(screen, "  No environments found on this machine.") ||
		!strings.Contains(screen, "  Create one with hm setup inside a Magento project.") {
		t.Errorf("%s", screen)
	}
}

func TestALongFleetScrollsAndSaysSo(t *testing.T) {
	l := lab(t, strings.Repeat("j", 20)+"q", 20)
	l.board.run()

	screen := l.screen()
	if !strings.Contains(screen, "  showing 6-20 of 20") {
		t.Errorf("%s", screen)
	}

	if !strings.Contains(screen, "> cms ") {
		t.Errorf("the selection is not on screen:\n%s", screen)
	}
}

func TestFramesArePaintedInPlace(t *testing.T) {
	l := lab(t, "q", 1)
	l.board.run()

	output := l.out.String()

	for _, sequence := range []string{"\033[?1049h", "\033[?25l", "\033[?2026h", "\033[1;1H", "\033[K",
		"\033[J", "\033[?2026l", "\033[24;1H", "\033[?25h", "\033[?1049l"} {
		if !strings.Contains(output, sequence) {
			t.Errorf("missing %q", sequence)
		}
	}

	if strings.Contains(output, "\033[2J") {
		t.Error("a frame erased the whole screen")
	}

	if !strings.HasSuffix(output, "\033[?25h\033[?1049l") {
		t.Errorf("does not end by giving the terminal back: %q", output[len(output)-30:])
	}
}

// ------------------------------------------------------------------ keys

func TestKeysAreNamed(t *testing.T) {
	cases := map[string]string{
		"\033[A": "up", "\033OA": "up", "\033[B": "down", "\033OB": "down", "\033[C": "right",
		"\033[D": "left", "\033": "esc", "\n": "enter", "\r": "enter", "\003": "ctrl-c",
		"\177": "backspace", "\010": "backspace", "\t": "tab", "q": "q", "?": "?",
	}

	for bytes, want := range cases {
		got, err := dashKey(bufio.NewReader(strings.NewReader(bytes)))
		if err != nil || got != want {
			t.Errorf("%q: %q, %v; want %q", bytes, got, err, want)
		}
	}

	if _, err := dashKey(bufio.NewReader(strings.NewReader(""))); err == nil {
		t.Error("no keys left is not an error")
	}
}

func TestDownAndUpMoveTheSelection(t *testing.T) {
	for keys, want := range map[string]string{
		"j": "blog", "\033[B": "blog", "jj": "store", "jjjj": "store", "jjk": "blog", "jj\033[A": "blog",
		"k": "shop",
	} {
		l := lab(t, keys+"q", 3)
		l.board.run()

		selected := ""
		for _, row := range l.lastFrame() {
			if strings.HasPrefix(row, "> ") {
				selected = strings.Fields(row)[1]
			}
		}

		if selected != want {
			t.Errorf("%q selected %q, want %q", keys, selected, want)
		}
	}
}

func TestQuittingKeys(t *testing.T) {
	for _, keys := range []string{"q", "\003", ""} {
		l := lab(t, keys+"jjj", 3)

		if code := l.board.run(); code != exitOK {
			t.Errorf("%q: exit %d", keys, code)
		}

		if keys != "" && l.board.selected != 0 {
			t.Errorf("%q: kept reading keys after quitting", keys)
		}
	}
}

func TestTheDataComesFromTheBinary(t *testing.T) {
	l := lab(t, "q", 1)
	l.board.run()

	if got := strings.Join(l.runner.argv(), ","); got != "list --json,doctor --json" {
		t.Errorf("%s", got)
	}
}

func TestRefreshReadsTheDataAgain(t *testing.T) {
	l := lab(t, "gq", 1)
	l.board.run()

	if got := strings.Join(l.runner.argv(), ","); got != "list --json,doctor --json,list --json,doctor --json" {
		t.Errorf("%s", got)
	}
}

func TestEnterOpensTheDetailWhereTheEnvironmentLives(t *testing.T) {
	l := lab(t, "j\rq", 2)
	l.board.run()

	last := l.runner.calls[len(l.runner.calls)-1]
	if last.args != "describe --json" || last.dir != l.roots[1] || last.combined {
		t.Errorf("%+v", last)
	}

	frame := l.lastFrame()
	if frame[0] != "blog" || frame[2] != "  Project    shop" {
		t.Errorf("%q", frame[:3])
	}

	if frame[23] != "esc back   s start   x stop   r restart   l logs   o browser   ? keys   q quit" {
		t.Errorf("footer %q", frame[23])
	}
}

func TestEnterOnAGoneDirectorySaysSo(t *testing.T) {
	l := lab(t, "", 1)
	l.runner.answers["list --json"] = fleetPayload(t, "/nonexistent/hm-tui")
	l.board.keys = bufio.NewReader(strings.NewReader("\rq"))
	l.board.run()

	frame := l.lastFrame()
	if frame[22] != "that environment's directory is gone" || frame[0] != "Dockergento — environments on this machine" {
		t.Errorf("%q / %q", frame[22], frame[0])
	}
}

func TestBackKeysReturnToTheFleet(t *testing.T) {
	for _, back := range []string{"\033", "h", "\177"} {
		l := lab(t, "\r"+back+"q", 1)
		l.board.run()

		if l.board.view != "fleet" {
			t.Errorf("%q left the view at %s", back, l.board.view)
		}
	}
}

func TestTheDetailScrolls(t *testing.T) {
	l := lab(t, "\r", 1)
	l.board.size = func() (int, int) { return 10, 80 }
	l.board.keys = bufio.NewReader(strings.NewReader("\rjjjq"))
	l.board.run()

	frame := l.lastFrame()
	if frame[2] != "  Magento    2.4.7  (developer)" || frame[7] != "  showing 4-8 of 14   j/k to scroll" {
		t.Errorf("%q", frame)
	}

	l = lab(t, "\rjjjjjjjjjjjjjjjjjjjjkq", 1)
	l.board.size = func() (int, int) { return 10, 80 }
	l.board.run()

	if got := l.lastFrame()[7]; got != "  showing 9-13 of 14   j/k to scroll" {
		t.Errorf("scrolling back from the end: %q", got)
	}
}

func TestActionsRunTheBinaryInTheEnvironment(t *testing.T) {
	cases := map[string]string{"s": "start", "x": "stop", "r": "restart", "l": "logs -f --tail 100"}

	for key, argv := range cases {
		l := lab(t, "j"+key+"q", 2)
		l.board.run()

		var handed *dashCall
		for at := range l.runner.calls {
			if l.runner.calls[at].handed {
				handed = &l.runner.calls[at]
			}
		}

		if handed == nil || handed.args != argv || handed.dir != l.roots[1] {
			t.Errorf("%s: %+v", key, handed)

			continue
		}

		output := l.out.String()
		if !strings.Contains(output, "hm "+argv+"  in blog\n\n") ||
			!strings.Contains(output, "Done. Press any key to go back to the dashboard.") {
			t.Errorf("%s: %q", key, output)
		}

		if l.pauses != 1 {
			t.Errorf("%s: waited for %d keys", key, l.pauses)
		}

		// The data is read again afterwards
		if got := strings.Join(l.runner.argv(), ","); !strings.HasSuffix(got, argv+",list --json,doctor --json") {
			t.Errorf("%s: %s", key, got)
		}
	}
}

func TestAFailedActionSaysItsExitCode(t *testing.T) {
	l := lab(t, "sq", 1)
	l.runner.handed = 6
	l.board.run()

	if !strings.Contains(l.out.String(), "Failed with exit code 6. Press any key to go back to the dashboard.") {
		t.Errorf("%q", l.out.String())
	}
}

func TestActionsActOnTheSelectionFromTheDetailToo(t *testing.T) {
	l := lab(t, "j\rsq", 2)
	l.board.run()

	last := l.runner.calls[len(l.runner.calls)-3]
	if !last.handed || last.args != "start" || last.dir != l.roots[1] {
		t.Errorf("%+v", last)
	}
}

func TestActionsNeedAnEnvironment(t *testing.T) {
	l := lab(t, "sxrlo\rq", 0)
	l.board.run()

	if got := strings.Join(l.runner.argv(), ","); got != "list --json,doctor --json" {
		t.Errorf("%s", got)
	}
}

func TestActionsOnAGoneDirectorySaySo(t *testing.T) {
	for _, key := range []string{"s", "o"} {
		l := lab(t, "", 1)
		l.runner.answers["list --json"] = fleetPayload(t, "/nonexistent/hm-tui")
		l.board.keys = bufio.NewReader(strings.NewReader(key + "q"))
		l.board.run()

		if got := l.lastFrame()[22]; got != "that environment's directory is gone" {
			t.Errorf("%s: %q", key, got)
		}

		if len(l.runner.calls) != 2 {
			t.Errorf("%s: ran %v", key, l.runner.argv())
		}
	}
}

func TestTheBrowserKeyAsksLaunch(t *testing.T) {
	l := lab(t, "oq", 1)
	l.runner.answers["launch --json"] = `{"ok":true,"data":{"target":"base","url":"https://shop.test/","opened":false}}`
	l.board.run()

	last := l.runner.calls[len(l.runner.calls)-1]
	if last.args != "launch --json" || last.dir != l.roots[0] || !last.combined {
		t.Errorf("%+v", last)
	}

	if got := l.lastFrame()[22]; got != "opened https://shop.test/" {
		t.Errorf("%q", got)
	}
}

func TestTheBrowserKeySaysWhyItCouldNot(t *testing.T) {
	l := lab(t, "oq", 1)
	l.runner.answers["launch --json"] = `{"ok":false,"error":{"message":"This project has no domain configured, so it has no address to open"}}`
	l.runner.fails["launch --json"] = true
	l.board.run()

	if got := l.lastFrame()[22]; got != "This project has no domain configured, so it has no address to open" {
		t.Errorf("%q", got)
	}

	l = lab(t, "oq", 1)
	l.runner.answers["launch --json"] = `{"ok":false}`
	l.runner.fails["launch --json"] = true
	l.board.run()

	if got := l.lastFrame()[22]; got != "could not open it" {
		t.Errorf("%q", got)
	}
}

func TestTheKeysScreen(t *testing.T) {
	l := lab(t, "?q", 1)
	l.board.run()

	output := l.out.String()
	for _, line := range []string{
		"Keys\n\n", "  up/down, k/j move through the list\n", "  enter        open the selected environment\n",
		"  esc          back to the fleet\n", "  s            start the environment\n", "  x            stop it\n",
		"  r            restart it\n", "  l            follow its logs\n", "  o            open it in the browser\n",
		"  g            refresh the data\n", "  q            quit\n", "\nPress any key to go back.\n",
	} {
		if !strings.Contains(output, line) {
			t.Errorf("missing %q", line)
		}
	}

	if l.pauses != 1 {
		t.Errorf("waited for %d keys", l.pauses)
	}
}

func TestUnknownKeysDoNothing(t *testing.T) {
	l := lab(t, "zZ\033[C\tq", 2)
	l.board.run()

	if l.board.view != "fleet" || l.board.selected != 0 || len(l.runner.calls) != 2 {
		t.Errorf("%s %d %v", l.board.view, l.board.selected, l.runner.argv())
	}
}
