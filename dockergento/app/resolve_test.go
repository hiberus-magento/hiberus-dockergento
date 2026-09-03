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

func (vcs) Dirty(string) bool                                  { return false }
func (vcs) Tracked(string) ([]string, error)                   { return nil, nil }
func (vcs) RemoveWorktree(string, string, bool) error          { return nil }
func (vcs) Prune(string) error                                 { return nil }
func (vcs) AddWorktree(string, string, string) (string, error) { return "", nil }

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
func (registry) WriteOverlay(string, string, string) error { return nil }
func (registry) Save(string, core.Worktree) error          { return nil }
func (registry) Parents() ([]string, error)                { return nil, nil }

func TestMainCheckoutTakesItsOwnProperties(t *testing.T) {
	resolver := Resolver{
		Properties: properties{"/code/shop": {"COMPOSE_PROJECT_NAME": "shop", "DOMAIN": "shop.test"}},
		VCS:        vcs{},
		Registry:   registry{},
	}

	project, err := resolver.Resolve("/code/shop")
	if err != nil {
		t.Fatalf("Resolve = %v, want no error", err)
	}

	if project.Name != "shop" || project.Domain != "shop.test" || project.Root != "/code/shop" {
		t.Fatalf("project = %+v, want the one on disk", project)
	}

	if project.IsWorktree() {
		t.Fatal("main checkout reads as a worktree, want it not to")
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
		t.Fatalf("Resolve = %v, want no error", err)
	}

	if project.Root != "/code/shop" || project.Name != "shop" {
		t.Fatalf("project = %+v, want it resolved against the main checkout", project)
	}

	if project.IsWorktree() {
		t.Fatal("an unregistered worktree reads as a branch environment, want it not to")
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
		t.Fatalf("Resolve = %v, want no error", err)
	}

	if project.Root != "/code/shop-worktrees/feature-x" {
		t.Fatalf("root = %q, want the worktree's own", project.Root)
	}

	if project.MagentoDir != "./src" {
		t.Fatalf("magento dir = %q, want the worktree's own and not the parent's", project.MagentoDir)
	}

	if project.Name != "shop-feature-x" {
		t.Fatalf("project name = %q, want the registered one", project.Name)
	}

	if project.Domain != "feature-x.shop.test" {
		t.Fatalf("domain = %q, want the registered one", project.Domain)
	}

	if project.Worktree.Parent != "shop" {
		t.Fatalf("parent = %q, want the checkout it was made from", project.Worktree.Parent)
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
		t.Fatalf("Resolve = %v, want no error", err)
	}

	if project.MagentoDir != "." {
		t.Fatalf("magento dir = %q, want the parent's where the worktree has none", project.MagentoDir)
	}
}

// A directory that is not a project is not an error: several commands run outside one, and
// deciding that here would take the decision away from them.
func TestADirectoryThatIsNotAProjectIsNotAnError(t *testing.T) {
	resolver := Resolver{Properties: properties{}, VCS: vcs{}, Registry: registry{}}

	project, err := resolver.Resolve("/tmp/cualquiera")
	if err != nil {
		t.Fatalf("Take = %v, want no error", err)
	}

	if project.Name != "" || project.MagentoDir != "." {
		t.Fatalf("project = %+v, want the one on disk", project)
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
		t.Fatalf("topology = %q, want the declared one", project.Topology)
	}

	resolver.Properties = properties{"/code/shop": {}}
	project, _ = resolver.Resolve("/code/shop")

	if project.Topology != core.Classic {
		t.Fatalf("topology with none declared = %q, want classic", project.Topology)
	}
}

func TestAFailureReadingTheMainPropertiesIsReported(t *testing.T) {
	resolver := Resolver{Properties: failingProperties{}, VCS: vcs{}, Registry: registry{}}

	if _, err := resolver.Resolve("/code/shop"); err == nil {
		t.Fatal("a read that failed = nil, want the error carried out")
	}
}
