package app

import (
	"errors"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/internal/core"
)

type engine struct {
	containers []core.Container
	err        error
}

func (e engine) Containers() ([]core.Container, error) { return e.containers, e.err }

type filesystem struct{ present map[string]bool }

func (f filesystem) IsDir(path string) bool  { return f.present[path] }
func (f filesystem) Exists(path string) bool { return f.present[path] }
func (f filesystem) Read(string) string      { return "" }

type branches struct{ names map[string]string }

func (b branches) Branch(dir string) string { return b.names[dir] }

func inventoryOf(containers []core.Container, present map[string]bool, names map[string]string) Inventory {
	return Inventory{
		Engine:   engine{containers: containers},
		FS:       filesystem{present: present},
		Branches: branches{names: names},
	}
}

func TestContainersAreGroupedIntoEnvironments(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "shop", ComposeService: "phpfpm", Project: "shop", Root: "/code/shop", Running: true},
		{ComposeProject: "shop", ComposeService: "db", Project: "shop", Root: "/code/shop", Running: true},
		{ComposeProject: "shop", ComposeService: "nginx", Project: "shop", Root: "/code/shop"},
	}, map[string]bool{"/code/shop": true}, map[string]string{"/code/shop": "main"})

	environments, err := inventory.Environments()
	if err != nil {
		t.Fatal(err)
	}

	if len(environments) != 1 {
		t.Fatalf("un proyecto es un entorno: %d", len(environments))
	}

	environment := environments[0]

	if environment.Containers.Total != 3 || environment.Containers.Running != 2 {
		t.Fatalf("cuenta inesperada: %+v", environment.Containers)
	}

	// Half up is its own state, not a rounding of the other two: it is where people get stuck
	if environment.Status != "partial" {
		t.Fatalf("estado inesperado: %s", environment.Status)
	}

	if environment.Branch != "main" {
		t.Fatalf("rama inesperada: %s", environment.Branch)
	}
}

func TestStatusIsRunningStoppedOrPartial(t *testing.T) {
	for _, this := range []struct {
		running, total int
		expected       string
	}{
		{0, 3, "stopped"},
		{3, 3, "running"},
		{1, 3, "partial"},
	} {
		if got := statusOf(this.running, this.total); got != this.expected {
			t.Fatalf("%d/%d debería ser %s, no %s", this.running, this.total, this.expected, got)
		}
	}
}

// An environment whose directory is gone is what `hm clean` collects. Telling it from a stopped
// one is the whole distinction: a stopped project is not rubbish, it is a stopped project.
func TestAnEnvironmentWhoseDirectoryIsGoneIsAnOrphan(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "gone", ComposeService: "phpfpm", Project: "gone", Root: "/code/gone"},
	}, map[string]bool{}, map[string]string{})

	environments, _ := inventory.Environments()

	if !environments[0].Orphan {
		t.Fatal("un directorio que no está es un huérfano")
	}

	// And nobody asked git about a directory that is not there
	if environments[0].Branch != "" {
		t.Fatalf("no debería haber rama: %s", environments[0].Branch)
	}
}

// Environments created before the labels existed still have to appear: something that is not in
// an inventory is something nobody ever cleans up.
func TestAProjectWithoutOurLabelsStillCountsIfItHasPHP(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "viejo", ComposeService: "phpfpm", WorkingDir: "/code/viejo"},
		{ComposeProject: "viejo", ComposeService: "db", WorkingDir: "/code/viejo"},
	}, map[string]bool{"/code/viejo": true}, map[string]string{})

	environments, _ := inventory.Environments()

	if len(environments) != 1 {
		t.Fatalf("debería contarse: %d", len(environments))
	}

	if environments[0].HasMetadata {
		t.Fatal("pero sin decir que lleva nuestras etiquetas")
	}

	if environments[0].Root != "/code/viejo" {
		t.Fatalf("la raíz sale de la de compose: %s", environments[0].Root)
	}
}

// Somebody else's compose project is not ours to list.
func TestSomethingElseEntirelyIsNotAnEnvironment(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "wordpress", ComposeService: "web"},
		{ComposeProject: "wordpress", ComposeService: "db"},
	}, map[string]bool{}, map[string]string{})

	environments, _ := inventory.Environments()

	if len(environments) != 0 {
		t.Fatalf("no es nuestro: %+v", environments)
	}
}

func TestAContainerWithNoProjectAtAllIsSkipped(t *testing.T) {
	inventory := inventoryOf([]core.Container{{ComposeService: "phpfpm"}}, nil, nil)

	environments, _ := inventory.Environments()
	if len(environments) != 0 {
		t.Fatalf("sin nombre no hay entorno: %+v", environments)
	}
}

// The first non-empty label wins: the containers of one environment carry the same values, and a
// later one with the label missing must not erase what an earlier one knew.
func TestALabelMissingOnOneContainerDoesNotEraseIt(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "shop", ComposeService: "phpfpm", Project: "shop", Root: "/code/shop", Worktree: "feature-x"},
		{ComposeProject: "shop", ComposeService: "db", Project: "shop"},
	}, map[string]bool{"/code/shop": true}, map[string]string{})

	environments, _ := inventory.Environments()

	if environments[0].Root != "/code/shop" || environments[0].Worktree != "feature-x" {
		t.Fatalf("se perdió una etiqueta: %+v", environments[0])
	}
}

// Sorted by bytes, so the order does not depend on the machine's locale — which is exactly what
// went wrong in the shell version, where `magento_dev` came before `magento-demo` under one
// locale and after it under another.
func TestTheOrderIsTheSameEverywhere(t *testing.T) {
	inventory := inventoryOf([]core.Container{
		{ComposeProject: "magento_dev", ComposeService: "phpfpm", Project: "magento_dev"},
		{ComposeProject: "magento-demo", ComposeService: "phpfpm", Project: "magento-demo"},
	}, nil, nil)

	environments, _ := inventory.Environments()

	if environments[0].Name != "magento-demo" {
		t.Fatalf("orden inesperado: %s antes que %s", environments[0].Name, environments[1].Name)
	}
}

func TestADaemonThatCannotBeReachedIsReported(t *testing.T) {
	inventory := Inventory{Engine: engine{err: errors.New("sin demonio")}, FS: filesystem{}, Branches: branches{}}

	if _, err := inventory.Environments(); err == nil {
		t.Fatal("debería propagarse")
	}
}
