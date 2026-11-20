package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

type orchestrator struct {
	upCalls    [][]string
	buildCalls [][]string
	stopCalls  [][]string
	downCalls  []core.DownOptions
	err        error

	// stopErr fails only the stop, for the operations that go on to something else after it.
	stopErr error

	// log, when set, receives every operation in the order it was asked, shared with the other
	// fakes of a test so that their interleaving is one sequence. exits answers an exec with its
	// exit code, by the command it ran; a command it does not name exits 0.
	log   *[]string
	exits map[string]int
}

func (o *orchestrator) record(entry string) {
	if o.log != nil {
		*o.log = append(*o.log, entry)
	}
}

func (o *orchestrator) Up(_ core.Project, _ core.ComposeFiles, services []string) error {
	o.upCalls = append(o.upCalls, services)
	o.record("up " + strings.Join(services, " "))

	return o.err
}

func (o *orchestrator) UpBuilding(_ core.Project, _ core.ComposeFiles, services []string) error {
	o.buildCalls = append(o.buildCalls, services)
	o.record("up --build " + strings.Join(services, " "))

	return o.err
}

func (o *orchestrator) Stop(_ core.Project, _ core.ComposeFiles, services []string) error {
	o.stopCalls = append(o.stopCalls, services)
	o.record("stop " + strings.Join(services, " "))

	if o.stopErr != nil {
		return o.stopErr
	}

	return o.err
}

func (o *orchestrator) Down(_ core.Project, _ core.ComposeFiles, options core.DownOptions) error {
	o.downCalls = append(o.downCalls, options)

	return o.err
}

func (o *orchestrator) Logs(core.Project, core.ComposeFiles, []string, core.LogOptions) error {
	return o.err
}

func (o *orchestrator) Exec(_ core.Project, _ core.ComposeFiles, service string, command []string, options core.ExecOptions) (int, error) {
	o.record(execEntry(options.User, service, command...))

	if o.err != nil {
		return 0, o.err
	}

	return o.exits[command[len(command)-1]], nil
}

type shell struct {
	ran  [][]string
	code int
	err  error
}

func (s *shell) Run(args []string) (int, error) {
	s.ran = append(s.ran, args)

	return s.code, s.err
}

// composeRunner is the fake for ports.ComposeRunner: it records what it was asked to run, rather
// than running Compose.
type composeRunner struct {
	dir         string
	files       []string
	environment map[string]string
	args        []string

	status int
	err    error
}

func (c *composeRunner) Run(dir string, files []string, environment map[string]string, args []string) (int, error) {
	c.dir, c.files, c.environment, c.args = dir, files, environment, args

	return c.status, c.err
}

// proxyStarter is the fake for the global proxy: it counts how many times it was started.
type proxyStarter struct {
	ups int
	err error
}

func (p *proxyStarter) Up() (bool, error) {
	p.ups++

	return p.err == nil, p.err
}

func operatorWith(containers []core.Container, orchestration *orchestrator) Operator {
	return Operator{
		Orchestrator: orchestration,
		Engine:       &engine{containers: containers},
		Proxy:        &proxyStarter{},
		Platform:     "mac",
		Binary:       "hm",
		Workdir:      "/var/www/html",

		// Never the real five seconds: a test on Linux that reaches the steps after starting
		// would otherwise sleep for them
		Wait: func(time.Duration) {},
	}
}

func refusalOf(t *testing.T, err error) core.Refusal {
	t.Helper()

	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want a refusal with a reason and a code", err)
	}

	return refusal
}

func TestTheProxyIsStartedForAProjectThatNeedsIt(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true, false); err != nil {
		t.Fatalf("Start = %v, want no error", err)
	}

	if proxy := operator.Proxy.(*proxyStarter); proxy.ups != 1 {
		t.Fatalf("proxy started %d times, want once without being asked", proxy.ups)
	}
}

func TestAProxyThatFailsToStartFailsTheStart(t *testing.T) {
	failure := errors.New("the proxy did not start")
	operator := operatorWith(nil, &orchestrator{})
	operator.Proxy = &proxyStarter{err: failure}

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true, false); !errors.Is(err, failure) {
		t.Fatalf("Start = %v, want the proxy's error", err)
	}
}

func TestAProxyAlreadyRunningIsLeftAlone(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith([]core.Container{{Name: "hm-proxy", Running: true}}, orchestration)

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true, false); err != nil {
		t.Fatalf("Start = %v, want no error", err)
	}

	if proxy := operator.Proxy.(*proxyStarter); proxy.ups != 0 {
		t.Fatalf("proxy started %d times, want it left alone where it is already running", proxy.ups)
	}
}

func TestSomethingElseHoldingThePortIsNamed(t *testing.T) {
	// "Port 80 is busy" is a fact; the name of the environment to go and stop is what somebody
	// can act on without asking anybody
	orchestration := &orchestrator{}
	operator := operatorWith([]core.Container{
		{Name: "otra-tienda-nginx-1", Running: true, Published: []string{"80", "443"}},
	}, orchestration)

	refusal := refusalOf(t, operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true, false))

	if refusal.Code != 6 {
		t.Fatalf("refusal code = %d, want the one for a deliberate refusal", refusal.Code)
	}

	if !strings.Contains(refusal.Message, "otra-tienda-nginx-1") {
		t.Fatalf("message = %q, want it to name what is in the way", refusal.Message)
	}

	if len(orchestration.upCalls) != 0 {
		t.Fatalf("the environment was started with the port taken, want nothing started")
	}
}

func TestDependenciesBoundFromTheHostAreRefused(t *testing.T) {
	//
	// PHP resolves __DIR__ through the mount and Composer builds every path from it: with the
	// dependencies bound from outside, the project's own modules are never registered and the
	// failure that follows names none of this.
	//
	orchestration := &orchestrator{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "bind", Destination: "/var/www/html/vendor"}},
	}}, orchestration)

	refusal := refusalOf(t, operator.Start(
		core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, nil, false, false, false))

	if !strings.Contains(refusal.Message, "/var/www/html/vendor") {
		t.Fatalf("message = %q, want it to name the mount", refusal.Message)
	}

	if refusal.Hint != "hm rebuild" {
		t.Fatalf("hint = %q, want it to say what to do", refusal.Hint)
	}
}

func TestAVolumeForTheDependenciesIsFine(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "volume", Destination: "/var/www/html/vendor"}},
	}}, orchestration)

	if err := operator.Start(core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("start with a named volume = %v, want no refusal", err)
	}
}

func TestNamingAServiceDoesNotDragInTheWholeCheck(t *testing.T) {
	// Naming a service is asking for that service, and answering with a complaint about a
	// different one is a command arguing with its caller
	orchestration := &orchestrator{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "bind", Destination: "/var/www/html/vendor"}},
	}}, orchestration)

	err := operator.Start(core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, []string{"db"}, false, false, false)
	if err != nil {
		t.Fatalf("start of one service = %v, want no refusal about the rest", err)
	}
}

func TestAFailedSnapshotLeavesTheEnvironmentRunning(t *testing.T) {
	// A stopped environment and no copy, after asking for one, is the worst of the three
	// possible outcomes
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	refusal := refusalOf(t, operator.Stop(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true))

	if refusal.Kind != "snapshot_failed" {
		t.Fatalf("refusal kind = %q, want the one for a copy that failed", refusal.Kind)
	}

	if len(orchestration.stopCalls) != 0 {
		t.Fatalf("the environment was stopped after the copy failed, want it left running")
	}
}

func TestStoppingWithoutACopyDoesNotAskForOne(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	if err := operator.Stop(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false); err != nil {
		t.Fatalf("Stop = %v, want no error", err)
	}

	if len(orchestration.stopCalls) != 1 {
		t.Fatalf("stop calls = %v, want one", orchestration.stopCalls)
	}
}

func TestRestartingIsAStopAndAStart(t *testing.T) {
	//
	// And not Compose's own restart, which restarts the containers exactly as they are: somebody
	// who edits the configuration and runs `restart` expects the change to be running afterwards.
	//
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	if err := operator.Restart(core.Project{Name: "shop"}, core.ComposeFiles{}, []string{"phpfpm"}, false); err != nil {
		t.Fatalf("Restart = %v, want no error", err)
	}

	if len(orchestration.stopCalls) != 1 || len(orchestration.upCalls) != 1 {
		t.Fatalf("restart = %v stops and %v ups, want one of each", orchestration.stopCalls, orchestration.upCalls)
	}

	if orchestration.upCalls[0][0] != "phpfpm" {
		t.Fatalf("services started = %v, want the one that was asked for", orchestration.upCalls[0])
	}
}

//
// What the platform needs after the environment is up is a hand-off, not a fork of the command:
// everything else about starting is the same code on both, which is what keeps them from
// drifting.
//

func TestLinuxRunsTheStepsAfterStartingItself(t *testing.T) {
	// No hand-off to the shell any more: the ids and the self-routing entries are this package's,
	// and Start runs them once the environment is up
	log := []string{}
	orchestration := &orchestrator{log: &log}
	operator := linuxOperator(&log, orchestration, "1000", "1000", nil)

	if err := operator.Start(linuxProject, core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("Start = %v, want no error", err)
	}

	if len(log) == 0 || log[0] != "up " {
		t.Fatalf("first thing done = %v, want the environment up before the steps", log)
	}

	if !containsEntry(log, execEntry("root", "phpfpm", "bash", "-c", `echo "172.18.0.9 localhost" >> /etc/hosts`)) {
		t.Fatalf("done on linux = %v, want the steps after starting run in Go", log)
	}
}

func TestMacOSIsNotEvenAsked(t *testing.T) {
	// Asking costs a shell process, on a command where that is a fifth of the time
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	if err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("Start = %v, want no error", err)
	}
}

func TestTheEnvironmentIsUpBeforeAnyOfThatIsTried(t *testing.T) {
	// The steps run after the up, so a project whose post-steps fail still has its containers
	// running — which is the state somebody can look at. Here they fail at their first step:
	// there is no container to read the ids from
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)
	operator.Platform = "linux"

	err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false, false)

	if err == nil {
		t.Fatal("a failure after starting = nil, want it carried out")
	}

	if len(orchestration.upCalls) != 1 {
		t.Fatalf("up calls = %v, want the environment started before the step that failed", orchestration.upCalls)
	}
}

//
// A worktree with no environment of its own resolves to the main checkout, which is right for
// reading and catastrophic for anything that recreates or destroys one: somebody standing in a
// branch does not expect `stop` to stop the environment they left running in the main checkout.
//
// The shell implementation refused this from the start. Porting start, stop and restart lost it,
// and nothing said so — no test ran from a worktree.
//

func enUnWorktreeSinRegistrar() core.Project {
	return core.Project{Name: "tienda", Root: "/code/tienda", InWorktree: true}
}

func TestUnaRamaSinEntornoNoPuedeArrancarElDelPrincipal(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	refusal := refusalOf(t, operator.Start(enUnWorktreeSinRegistrar(), core.ComposeFiles{}, nil, false, false, false))

	if refusal.Code != 6 || refusal.Kind != "blocked_in_worktree" {
		t.Fatalf("refusal = %+v, want a deliberate one with its code", refusal)
	}

	if !strings.Contains(refusal.Message, "/code/tienda") {
		t.Fatalf("message = %q, want it to say which environment", refusal.Message)
	}

	if len(orchestration.upCalls) != 0 {
		t.Fatal("something was started, want nothing")
	}
}

func TestNiPararloNiReiniciarlo(t *testing.T) {
	for _, hacer := range []string{"stop", "restart"} {
		orchestration := &orchestrator{}
		operator := operatorWith(nil, orchestration)

		var err error

		switch hacer {
		case "stop":
			err = operator.Stop(enUnWorktreeSinRegistrar(), core.ComposeFiles{}, nil, false)
		case "restart":
			err = operator.Restart(enUnWorktreeSinRegistrar(), core.ComposeFiles{}, nil, false)
		}

		if refusal := refusalOf(t, err); refusal.Code != 6 {
			t.Fatalf("%s = %+v, want a refusal", hacer, refusal)
		}

		if len(orchestration.stopCalls) != 0 {
			t.Fatalf("%s touched the environment, want nothing touched", hacer)
		}
	}
}

func TestUnaRamaConEntornoPropioHaceLoQueQuiere(t *testing.T) {
	// Tiene sus contenedores, sus volúmenes y sus montajes: no hay nada de lo que proteger al
	// principal
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	proyecto := enUnWorktreeSinRegistrar()
	proyecto.Worktree = &core.Worktree{Name: "rama", Parent: "tienda"}

	if err := operator.Start(proyecto, core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("start from a registered branch environment = %v, want no refusal", err)
	}
}

func TestForzarLoLevantaParaUnaInvocacion(t *testing.T) {
	// Es una decisión, no un ajuste: no hay variable ni fichero que apague la guarda para siempre
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)
	operator.Forced = true

	if err := operator.Start(enUnWorktreeSinRegistrar(), core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("start with --force = %v, want no refusal", err)
	}
}

func TestFueraDeUnWorktreeNoHayNadaQueGuardar(t *testing.T) {
	orchestration := &orchestrator{}
	operator := operatorWith(nil, orchestration)

	if err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false, false); err != nil {
		t.Fatalf("start from a main checkout = %v, want no refusal", err)
	}
}

//
// Stopping every container on the machine. hm start -s and hm docker-stop-all ask the same
// question, in the same words, before stopping anybody's containers — so the confirmation lives
// once, in StopEverything, and Start calls it where it used to call o.Legacy.Run.
//

// asker is a question with a free-form answer, the fake for Operator.Ask.
type asker struct {
	questions []string
	answer    string
	err       error
}

func (a *asker) Ask(question, _ string) (string, error) {
	a.questions = append(a.questions, question)

	return a.answer, a.err
}

func TestConfirmingStopsThem(t *testing.T) {
	fakeEngine := &engine{containers: []core.Container{
		{ID: "a", ComposeProject: "shop", Running: true},
		{ID: "b", ComposeProject: "shop", Running: true},
		{ID: "c", ComposeProject: "other", Running: true},
	}}
	asked := &asker{answer: "y"}

	operator := Operator{Engine: fakeEngine, Ask: asked.Ask}

	result, err := operator.StopEverything(core.Project{Name: "shop"}, true)
	if err != nil {
		t.Fatalf("StopEverything = %v, want no error", err)
	}

	if len(fakeEngine.stopped) != 3 {
		t.Fatalf("stopped = %v, want every running container asked to stop", fakeEngine.stopped)
	}

	if result.Stopped != len(fakeEngine.stopped) {
		t.Fatalf("result.Stopped = %d, want it to match the count actually stopped", result.Stopped)
	}
}

func TestTheIdsAskedForAreTheIdsFoundRunning(t *testing.T) {
	fakeEngine := &engine{containers: []core.Container{
		{ID: "running-1", Running: true},
		{ID: "stopped-1", Running: false},
		{ID: "running-2", Running: true},
	}}
	asked := &asker{answer: "y"}

	operator := Operator{Engine: fakeEngine, Ask: asked.Ask}

	if _, err := operator.StopEverything(core.Project{Name: "shop"}, true); err != nil {
		t.Fatalf("StopEverything = %v, want no error", err)
	}

	if len(fakeEngine.stopped) != 2 || fakeEngine.stopped[0] != "running-1" || fakeEngine.stopped[1] != "running-2" {
		t.Fatalf("ids asked to stop = %v, want exactly the ones found running", fakeEngine.stopped)
	}
}

func TestTheDoesNotBelongLineIsConditional(t *testing.T) {
	t.Run("nobody else's", func(t *testing.T) {
		announced := []string{}
		operator := Operator{
			Engine:   &engine{containers: []core.Container{{ID: "a", ComposeProject: "shop", Running: true}}},
			Ask:      (&asker{answer: "y"}).Ask,
			Announce: func(message string) { announced = append(announced, message) },
		}

		if _, err := operator.StopEverything(core.Project{Name: "shop"}, true); err != nil {
			t.Fatalf("StopEverything = %v, want no error", err)
		}

		for _, line := range announced {
			if strings.Contains(line, "do not belong") {
				t.Fatalf("announced = %v, want no line about containers that are not the caller's", announced)
			}
		}
	})

	t.Run("some of them", func(t *testing.T) {
		announced := []string{}
		operator := Operator{
			Engine: &engine{containers: []core.Container{
				{ID: "a", ComposeProject: "shop", Running: true},
				{ID: "b", ComposeProject: "other", Running: true},
			}},
			Ask:      (&asker{answer: "y"}).Ask,
			Announce: func(message string) { announced = append(announced, message) },
		}

		if _, err := operator.StopEverything(core.Project{Name: "shop"}, true); err != nil {
			t.Fatalf("StopEverything = %v, want no error", err)
		}

		found := false

		for _, line := range announced {
			if strings.Contains(line, "1 of them do not belong to 'shop'.") {
				found = true
			}
		}

		if !found {
			t.Fatalf("announced = %v, want the line naming what does not belong", announced)
		}
	})
}

func TestSomeContainersRefuseToStop(t *testing.T) {
	fakeEngine := &engine{
		containers: []core.Container{
			{ID: "a", ComposeProject: "shop", Running: true},
			{ID: "b", ComposeProject: "shop", Running: true},
		},
		failed: []string{"b"},
	}
	asked := &asker{answer: "y"}

	operator := Operator{Engine: fakeEngine, Ask: asked.Ask}

	result, err := operator.StopEverything(core.Project{Name: "shop"}, true)
	if err == nil {
		t.Fatal("StopEverything with a refusal = nil, want the failure carried out")
	}

	if !strings.Contains(err.Error(), "b") {
		t.Fatalf("error = %v, want it to name the container that refused", err)
	}

	if len(fakeEngine.stopped) != 2 {
		t.Fatalf("stopped = %v, want every container still asked, refusal or not", fakeEngine.stopped)
	}

	if result.Stopped != 1 {
		t.Fatalf("result.Stopped = %d, want the count that actually stopped", result.Stopped)
	}
}

func TestStartingWithMinusSAsksTheSameQuestion(t *testing.T) {
	t.Run("interactive", func(t *testing.T) {
		orchestration := &orchestrator{}
		fakeEngine := &engine{containers: []core.Container{{ID: "a", ComposeProject: "other", Running: true}}}
		asked := &asker{answer: "y"}

		operator := operatorWith(nil, orchestration)
		operator.Engine = fakeEngine
		operator.Ask = asked.Ask

		if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true, false, true); err != nil {
			t.Fatalf("Start with -s = %v, want no error", err)
		}

		if len(asked.questions) != 1 || asked.questions[0] != "Stop them all? [y/N]:" {
			t.Fatalf("asked = %v, want the same question docker-stop-all asks", asked.questions)
		}
	})

	t.Run("non-interactive", func(t *testing.T) {
		orchestration := &orchestrator{}
		fakeEngine := &engine{containers: []core.Container{{ID: "a", ComposeProject: "other", Running: true}}}
		asked := &asker{answer: "y"}

		operator := operatorWith(nil, orchestration)
		operator.Engine = fakeEngine
		operator.Ask = asked.Ask

		if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true, false, false); err != nil {
			t.Fatalf("Start with -s, non-interactive = %v, want no error", err)
		}

		if len(asked.questions) != 0 {
			t.Fatalf("asked = %v, want nothing asked when non-interactive", asked.questions)
		}

		if len(fakeEngine.stopped) != 1 {
			t.Fatalf("stopped = %v, want it stopped without asking", fakeEngine.stopped)
		}
	})
}

func TestAnAPICallerIsNeverAsked(t *testing.T) {
	orchestration := &orchestrator{}
	fakeEngine := &engine{containers: []core.Container{{ID: "a", Running: true}}}

	operator := operatorWith(nil, orchestration)
	operator.Engine = fakeEngine
	operator.Ask = func(string, string) (string, error) {
		t.Fatal("Ask was called, want an API caller never asked")

		return "", nil
	}

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true, false, false); err != nil {
		t.Fatalf("Start non-interactively = %v, want no error", err)
	}
}

func TestStoppingTheRestBeforeStarting(t *testing.T) {
	orchestration := &orchestrator{}
	fakeEngine := &engine{containers: []core.Container{{ID: "a", ComposeProject: "other", Running: true}}}

	operator := operatorWith(nil, orchestration)
	operator.Engine = fakeEngine

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true, false, false); err != nil {
		t.Fatalf("Start with -s = %v, want no error", err)
	}

	if len(fakeEngine.stopped) != 1 {
		t.Fatalf("stopped = %v, want the rest stopped before starting", fakeEngine.stopped)
	}

	if len(orchestration.upCalls) != 1 {
		t.Fatalf("up calls = %v, want the environment started after stopping the rest", orchestration.upCalls)
	}
}

func TestStoppingTheRestIsTheSameCallOnEitherPlatform(t *testing.T) {
	for _, platform := range []string{"mac", "linux"} {
		t.Run(platform, func(t *testing.T) {
			log := []string{}
			orchestration := &orchestrator{log: &log}

			// On Linux the steps after starting need the project's own containers to read the
			// ids from; they are stopped with the rest, which this fake does not act on
			operator := linuxOperator(&log, orchestration, "1000", "1000", nil)
			fakeEngine := &engine{containers: append(
				[]core.Container{{ID: "a", ComposeProject: "other", Running: true}},
				runningLinuxStack("nginx", "phpfpm")...)}
			operator.Engine = fakeEngine
			operator.Platform = platform

			if err := operator.Start(linuxProject, core.ComposeFiles{}, nil, true, false, false); err != nil {
				t.Fatalf("Start with -s on %s = %v, want no error", platform, err)
			}

			if len(fakeEngine.stopped) == 0 || fakeEngine.stopped[0] != "a" {
				t.Fatalf("stopped on %s = %v, want the same in-process call", platform, fakeEngine.stopped)
			}
		})
	}
}

//
// docker-compose: an arbitrary Compose subcommand run through the tool, against exactly the files
// and environment the project resolves to. composelib implements operations, not a command line,
// so this is a subprocess — and the fake here proves what it is asked to run without one.
//

func TestWhatTheComposeRunnerIsAskedToRun(t *testing.T) {
	runner := &composeRunner{status: 0}
	operator := Operator{ComposeRunner: runner}

	files := []string{"/code/shop/docker-compose.yml", "/code/shop/docker-compose.dev.mac.yml"}
	environment := map[string]string{"COMPOSE_PROJECT_NAME": "shop"}
	args := []string{"config", "--format", "json"}

	status, err := operator.Compose(core.Project{Name: "shop", Root: "/code/shop"}, files, environment, args)
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}

	if status != 0 {
		t.Fatalf("status = %d, want the runner's own exit code", status)
	}

	if runner.dir != "/code/shop" {
		t.Fatalf("dir = %q, want the project's root", runner.dir)
	}

	if len(runner.files) != 2 || runner.files[0] != files[0] || runner.files[1] != files[1] {
		t.Fatalf("files = %v, want the resolved file list, in order", runner.files)
	}

	if runner.environment["COMPOSE_PROJECT_NAME"] != "shop" {
		t.Fatalf("environment = %v, want the project's own", runner.environment)
	}

	if len(runner.args) != 3 || runner.args[2] != "json" {
		t.Fatalf("args = %v, want them passed through verbatim", runner.args)
	}
}

//
// The two answers that stop nothing.
//
// Both are the whole point of asking: the question exists so that no can be an answer, and a
// machine with nothing up should not be asked at all. Neither reaches the engine, which is what
// these pin — a port that stopped things anyway would still pass every other test here.
//

func TestDecliningStopsNothing(t *testing.T) {
	fakeEngine := &engine{containers: []core.Container{
		{ID: "a", ComposeProject: "shop", Running: true},
		{ID: "b", ComposeProject: "other", Running: true},
	}}
	asked := &asker{answer: "n"}
	announced := []string{}

	operator := Operator{
		Engine:   fakeEngine,
		Ask:      asked.Ask,
		Announce: func(message string) { announced = append(announced, message) },
	}

	result, err := operator.StopEverything(core.Project{Name: "shop"}, true)
	if err != nil {
		t.Fatalf("StopEverything = %v, want no error: declining is an answer, not a failure", err)
	}

	if fakeEngine.stopped != nil {
		t.Fatalf("stopped = %v, want the engine never asked to stop anything", fakeEngine.stopped)
	}

	if result.Stopped != 0 {
		t.Fatalf("result.Stopped = %d, want nothing counted as stopped", result.Stopped)
	}

	if result.Total != 2 {
		t.Fatalf("result.Total = %d, want what was found running to still be reported", result.Total)
	}

	found := false

	for _, line := range announced {
		if strings.Contains(line, "Nothing was stopped.") {
			found = true
		}
	}

	if !found {
		t.Fatalf("announced = %v, want it to say nothing was stopped", announced)
	}
}

func TestWithNothingRunningNobodyIsAsked(t *testing.T) {
	fakeEngine := &engine{containers: []core.Container{
		{ID: "a", ComposeProject: "shop", Running: false},
	}}
	asked := &asker{answer: "y"}
	announced := []string{}

	operator := Operator{
		Engine:   fakeEngine,
		Ask:      asked.Ask,
		Announce: func(message string) { announced = append(announced, message) },
	}

	result, err := operator.StopEverything(core.Project{Name: "shop"}, true)
	if err != nil {
		t.Fatalf("StopEverything = %v, want no error", err)
	}

	if len(asked.questions) != 0 {
		t.Fatalf("questions = %v, want none: there is nothing to decide about", asked.questions)
	}

	if fakeEngine.stopped != nil {
		t.Fatalf("stopped = %v, want the engine never asked to stop anything", fakeEngine.stopped)
	}

	if result.Total != 0 || result.Stopped != 0 {
		t.Fatalf("result = %+v, want nothing found and nothing stopped", result)
	}

	if len(announced) != 1 || !strings.Contains(announced[0], "No containers running") {
		t.Fatalf("announced = %v, want only the line saying nothing is up", announced)
	}
}
