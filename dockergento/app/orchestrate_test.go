package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

type orchestrator struct {
	upCalls   [][]string
	stopCalls [][]string
	err       error
}

func (o *orchestrator) Up(_ core.Project, _ core.ComposeFiles, services []string) error {
	o.upCalls = append(o.upCalls, services)

	return o.err
}

func (o *orchestrator) Stop(_ core.Project, _ core.ComposeFiles, services []string) error {
	o.stopCalls = append(o.stopCalls, services)

	return o.err
}

func (o *orchestrator) Logs(core.Project, core.ComposeFiles, []string, core.LogOptions) error {
	return o.err
}

func (o *orchestrator) Exec(core.Project, core.ComposeFiles, string, []string, core.ExecOptions) (int, error) {
	return 0, o.err
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

func operatorWith(containers []core.Container, orchestration *orchestrator, legacy *shell) Operator {
	return Operator{
		Orchestrator: orchestration,
		Engine:       engine{containers: containers},
		Legacy:       legacy,
		Platform:     "mac",
		Binary:       "hm",
		Workdir:      "/var/www/html",
	}
}

func refusalOf(t *testing.T, err error) core.Refusal {
	t.Helper()

	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("se esperaba una negativa con motivo y código, y llegó: %v", err)
	}

	return refusal
}

func TestTheProxyIsStartedForAProjectThatNeedsIt(t *testing.T) {
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith(nil, orchestration, legacy)

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true); err != nil {
		t.Fatalf("arranque fallido: %v", err)
	}

	if len(legacy.ran) != 1 || strings.Join(legacy.ran[0], " ") != "proxy up" {
		t.Fatalf("nadie debería tener que acordarse de levantar el proxy: %v", legacy.ran)
	}
}

func TestAProxyAlreadyRunningIsLeftAlone(t *testing.T) {
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith([]core.Container{{Name: "hm-proxy", Running: true}}, orchestration, legacy)

	if err := operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true); err != nil {
		t.Fatalf("arranque fallido: %v", err)
	}

	if len(legacy.ran) != 0 {
		t.Fatalf("el proxy ya estaba en marcha: %v", legacy.ran)
	}
}

func TestSomethingElseHoldingThePortIsNamed(t *testing.T) {
	// "Port 80 is busy" is a fact; the name of the environment to go and stop is what somebody
	// can act on without asking anybody
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith([]core.Container{
		{Name: "otra-tienda-nginx-1", Running: true, Published: []string{"80", "443"}},
	}, orchestration, legacy)

	refusal := refusalOf(t, operator.Start(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false, true))

	if refusal.Code != 6 {
		t.Fatalf("una negativa a propósito tiene su propio código, y llegó %d", refusal.Code)
	}

	if !strings.Contains(refusal.Message, "otra-tienda-nginx-1") {
		t.Fatalf("el mensaje tiene que nombrar al culpable: %q", refusal.Message)
	}

	if len(orchestration.upCalls) != 0 {
		t.Fatalf("no se arranca nada cuando el puerto que hace falta está cogido")
	}
}

func TestDependenciesBoundFromTheHostAreRefused(t *testing.T) {
	//
	// PHP resolves __DIR__ through the mount and Composer builds every path from it: with the
	// dependencies bound from outside, the project's own modules are never registered and the
	// failure that follows names none of this.
	//
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "bind", Destination: "/var/www/html/vendor"}},
	}}, orchestration, legacy)

	refusal := refusalOf(t, operator.Start(
		core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, nil, false, false))

	if !strings.Contains(refusal.Message, "/var/www/html/vendor") {
		t.Fatalf("el mensaje tiene que decir qué montaje es: %q", refusal.Message)
	}

	if refusal.Hint != "hm rebuild" {
		t.Fatalf("y qué hacer con él: %q", refusal.Hint)
	}
}

func TestAVolumeForTheDependenciesIsFine(t *testing.T) {
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "volume", Destination: "/var/www/html/vendor"}},
	}}, orchestration, legacy)

	if err := operator.Start(core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, nil, false, false); err != nil {
		t.Fatalf("un volumen con nombre es exactamente lo que se quiere: %v", err)
	}
}

func TestNamingAServiceDoesNotDragInTheWholeCheck(t *testing.T) {
	// Naming a service is asking for that service, and answering with a complaint about a
	// different one is a command arguing with its caller
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith([]core.Container{{
		Project: "shop", ComposeService: "phpfpm", Running: true,
		Mounts: []core.Mount{{Type: "bind", Destination: "/var/www/html/vendor"}},
	}}, orchestration, legacy)

	err := operator.Start(core.Project{Name: "shop", MagentoDir: "."}, core.ComposeFiles{}, []string{"db"}, false, false)
	if err != nil {
		t.Fatalf("se pidió un servicio, no el entorno entero: %v", err)
	}
}

func TestAFailedSnapshotLeavesTheEnvironmentRunning(t *testing.T) {
	// A stopped environment and no copy, after asking for one, is the worst of the three
	// possible outcomes
	orchestration, legacy := &orchestrator{}, &shell{code: 1}
	operator := operatorWith(nil, orchestration, legacy)

	refusal := refusalOf(t, operator.Stop(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, true))

	if refusal.Kind != "snapshot_failed" {
		t.Fatalf("motivo equivocado: %q", refusal.Kind)
	}

	if len(orchestration.stopCalls) != 0 {
		t.Fatalf("no se para nada si la copia que se pidió no se hizo")
	}
}

func TestStoppingWithoutACopyDoesNotAskForOne(t *testing.T) {
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith(nil, orchestration, legacy)

	if err := operator.Stop(core.Project{Name: "shop"}, core.ComposeFiles{}, nil, false); err != nil {
		t.Fatalf("parada fallida: %v", err)
	}

	if len(legacy.ran) != 0 {
		t.Fatalf("un `stop` que a veces tarda un minuto es una sorpresa desagradable: %v", legacy.ran)
	}

	if len(orchestration.stopCalls) != 1 {
		t.Fatalf("y tiene que parar: %v", orchestration.stopCalls)
	}
}

func TestRestartingIsAStopAndAStart(t *testing.T) {
	//
	// And not Compose's own restart, which restarts the containers exactly as they are: somebody
	// who edits the configuration and runs `restart` expects the change to be running afterwards.
	//
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith(nil, orchestration, legacy)

	if err := operator.Restart(core.Project{Name: "shop"}, core.ComposeFiles{}, []string{"phpfpm"}, false); err != nil {
		t.Fatalf("reinicio fallido: %v", err)
	}

	if len(orchestration.stopCalls) != 1 || len(orchestration.upCalls) != 1 {
		t.Fatalf("un reinicio es una parada y un arranque: %v / %v", orchestration.stopCalls, orchestration.upCalls)
	}

	if orchestration.upCalls[0][0] != "phpfpm" {
		t.Fatalf("y sobre el servicio que se pidió: %v", orchestration.upCalls[0])
	}
}

//
// What the platform needs after the environment is up is a hand-off, not a fork of the command:
// everything else about starting is the same code on both, which is what keeps them from
// drifting.
//

func TestLinuxIsHandedBackWhatIsNotPortedYet(t *testing.T) {
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith(nil, orchestration, legacy)
	operator.Platform = "linux"

	if err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false); err != nil {
		t.Fatalf("arranque fallido: %v", err)
	}

	if len(legacy.ran) != 1 || strings.Join(legacy.ran[0], " ") != "post-start" {
		t.Fatalf("en Linux hay que igualar los ids y escribir el /etc/hosts del contenedor: %v", legacy.ran)
	}
}

func TestMacOSIsNotEvenAsked(t *testing.T) {
	// Asking costs a shell process, on a command where that is a fifth of the time
	orchestration, legacy := &orchestrator{}, &shell{}
	operator := operatorWith(nil, orchestration, legacy)

	if err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false); err != nil {
		t.Fatalf("arranque fallido: %v", err)
	}

	if len(legacy.ran) != 0 {
		t.Fatalf("en macOS no hay nada que hacer después: %v", legacy.ran)
	}
}

func TestTheEnvironmentIsUpBeforeAnyOfThatIsTried(t *testing.T) {
	// The hand-off happens after the up, so a project whose post-steps fail still has its
	// containers running — which is the state somebody can look at
	orchestration, legacy := &orchestrator{}, &shell{err: errors.New("no se pudo")}
	operator := operatorWith(nil, orchestration, legacy)
	operator.Platform = "linux"

	err := operator.Start(core.Project{Name: "tienda"}, core.ComposeFiles{}, nil, false, false)

	if err == nil {
		t.Fatal("un fallo después de arrancar sigue siendo un fallo")
	}

	if len(orchestration.upCalls) != 1 {
		t.Fatalf("pero el entorno ya está en marcha: %v", orchestration.upCalls)
	}
}
