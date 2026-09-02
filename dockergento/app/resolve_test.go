package app

import (
	"errors"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// The fakes are the point of the arrangement: resolving a project is exercised without a git
// repository, a filesystem or a Docker daemon, in microseconds. The shell implementation could
// only be tested against a real checkout, which is why the defect below went unnoticed for so
// long.

type properties map[string]map[string]string

func (p properties) Load(dir string) (map[string]string, error) {
	if values, ok := p[dir]; ok {
		return values, nil
	}

	return map[string]string{}, nil
}

func (properties) Set(string, string, string) error { return nil }

type failingProperties struct{}

func (failingProperties) Load(string) (map[string]string, error) {
	return nil, errors.New("no se puede leer")
}

func (failingProperties) Set(string, string, string) error { return nil }

type vcs struct {
	mainRoot   string
	isWorktree bool
	name       string
}

func (vcs) Dirty(string) bool                         { return false }
func (vcs) RemoveWorktree(string, string, bool) error { return nil }
func (vcs) Prune(string) error                        { return nil }

func (v vcs) Resolve(dir string) (string, bool, string, error) {
	if v.mainRoot == "" {
		return dir, false, "", nil
	}

	return v.mainRoot, v.isWorktree, v.name, nil
}

type registry struct {
	worktree *core.Worktree
}

func (r registry) Worktree(string, string) (*core.Worktree, error) { return r.worktree, nil }

func (registry) Worktrees(string) ([]core.Worktree, error) { return nil, nil }
func (registry) Forget(string, string) error               { return nil }
func (registry) Overlay(string, string) string             { return "" }

func TestMainCheckoutTakesItsOwnProperties(t *testing.T) {
	resolver := Resolver{
		Properties: properties{"/code/shop": {"COMPOSE_PROJECT_NAME": "shop", "DOMAIN": "shop.test"}},
		VCS:        vcs{},
		Registry:   registry{},
	}

	project, err := resolver.Resolve("/code/shop")
	if err != nil {
		t.Fatalf("resolución fallida: %v", err)
	}

	if project.Name != "shop" || project.Domain != "shop.test" || project.Root != "/code/shop" {
		t.Fatalf("proyecto inesperado: %+v", project)
	}

	if project.IsWorktree() {
		t.Fatal("un checkout principal no es un worktree")
	}
}

// A worktree with no environment of its own keeps resolving against the main checkout. That is
// what the refusals depend on: run `start` there and it would recreate the main environment with
// this directory's mounts.
func TestWorktreeWithoutEnvironmentResolvesAgainstTheMainCheckout(t *testing.T) {
	resolver := Resolver{
		Properties: properties{"/code/shop": {"COMPOSE_PROJECT_NAME": "shop", "DOMAIN": "shop.test"}},
		VCS:        vcs{mainRoot: "/code/shop", isWorktree: true, name: "feature-x"},
		Registry:   registry{},
	}

	project, err := resolver.Resolve("/code/shop-worktrees/feature-x")
	if err != nil {
		t.Fatalf("resolución fallida: %v", err)
	}

	if project.Root != "/code/shop" || project.Name != "shop" {
		t.Fatalf("debería resolver contra el principal: %+v", project)
	}

	if project.IsWorktree() {
		t.Fatal("sin registro no es un entorno de rama")
	}
}

// And one with an environment resolves against itself — including its properties, which is the
// defect the shell version carried: it read, and saved, the main checkout's.
func TestRegisteredWorktreeResolvesAgainstItself(t *testing.T) {
	resolver := Resolver{
		Properties: properties{
			"/code/shop":                     {"COMPOSE_PROJECT_NAME": "shop", "DOMAIN": "shop.test", "MAGENTO_DIR": "."},
			"/code/shop-worktrees/feature-x": {"COMPOSE_PROJECT_NAME": "shop", "DOMAIN": "shop.test", "MAGENTO_DIR": "./src"},
		},
		VCS:      vcs{mainRoot: "/code/shop", isWorktree: true, name: "feature-x"},
		Registry: registry{worktree: &core.Worktree{Name: "feature-x", Branch: "feature/x", Profile: "agent"}},
	}

	project, err := resolver.Resolve("/code/shop-worktrees/feature-x")
	if err != nil {
		t.Fatalf("resolución fallida: %v", err)
	}

	if project.Root != "/code/shop-worktrees/feature-x" {
		t.Fatalf("la raíz debería ser el worktree: %s", project.Root)
	}

	if project.MagentoDir != "./src" {
		t.Fatalf("las properties deberían ser las suyas, no las del padre: %s", project.MagentoDir)
	}

	if project.Name != "shop-feature-x" {
		t.Fatalf("nombre de proyecto inesperado: %s", project.Name)
	}

	if project.Domain != "feature-x.shop.test" {
		t.Fatalf("dirección inesperada: %s", project.Domain)
	}

	if project.Worktree.Parent != "shop" {
		t.Fatalf("debería recordar de quién viene: %s", project.Worktree.Parent)
	}
}

// A worktree whose properties cannot be read is a worktree of the same commit, so the main
// checkout's are the right answer. Refusing to work at all would be worse.
func TestUnreadablePropertiesFallBackRatherThanFail(t *testing.T) {
	resolver := Resolver{
		Properties: properties{"/code/shop": {"COMPOSE_PROJECT_NAME": "shop", "MAGENTO_DIR": "."}},
		VCS:        vcs{mainRoot: "/code/shop", isWorktree: true, name: "feature-x"},
		Registry:   registry{worktree: &core.Worktree{Name: "feature-x"}},
	}

	project, err := resolver.Resolve("/code/shop-worktrees/feature-x")
	if err != nil {
		t.Fatalf("resolución fallida: %v", err)
	}

	if project.MagentoDir != "." {
		t.Fatalf("debería caer en las del padre: %s", project.MagentoDir)
	}
}

// A directory that is not a project is not an error: several commands run outside one, and
// deciding that here would take the decision away from them.
func TestADirectoryThatIsNotAProjectIsNotAnError(t *testing.T) {
	resolver := Resolver{Properties: properties{}, VCS: vcs{}, Registry: registry{}}

	project, err := resolver.Resolve("/tmp/cualquiera")
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	if project.Name != "" || project.MagentoDir != "." {
		t.Fatalf("proyecto inesperado: %+v", project)
	}
}

func TestATopologyIsClassicUnlessItSaysOtherwise(t *testing.T) {
	resolver := Resolver{
		Properties: properties{"/code/shop": {"TOPOLOGY": "orchestrated"}},
		VCS:        vcs{},
		Registry:   registry{},
	}

	project, _ := resolver.Resolve("/code/shop")
	if project.Topology != core.Orchestrated {
		t.Fatalf("topología inesperada: %s", project.Topology)
	}

	resolver.Properties = properties{"/code/shop": {}}
	project, _ = resolver.Resolve("/code/shop")

	if project.Topology != core.Classic {
		t.Fatalf("por defecto es clásica: %s", project.Topology)
	}
}

func TestAFailureReadingTheMainPropertiesIsReported(t *testing.T) {
	resolver := Resolver{Properties: failingProperties{}, VCS: vcs{}, Registry: registry{}}

	if _, err := resolver.Resolve("/code/shop"); err == nil {
		t.Fatal("un fallo de lectura no puede pasar en silencio")
	}
}
