// Package hmstate reads what the tool records outside any checkout.
//
// Outside because config/docker/properties.json is a committed file: a branch environment's
// project name written there would travel in somebody's commit and rename the main environment
// at the same time. The registration is also the switch — a worktree with none keeps the
// guardrails that stop it from recreating the main environment with its own mounts.
package hmstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Registry is ~/.hm/worktrees.
type Registry struct {
	// Dir overrides where the registrations live. Empty means the default.
	Dir string
}

type registration struct {
	// Parent is redundant — it is the directory the file sits in — and it is written anyway,
	// because the shell implementation writes it and a record the two disagree about is a record
	// somebody will one day read with the wrong one.
	Parent  string `json:"parent,omitempty"`
	Created string `json:"created,omitempty"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Profile string `json:"profile"`
	Domain  string `json:"domain"`
	Project string `json:"project"`
	Vendor  string `json:"vendor"`
}

// Worktree returns the registration of a branch environment, or nil when there is none.
//
// Nil is not an error and is the answer that matters most: it is the difference between a
// worktree with an environment of its own and a worktree borrowing the main one's identity.
func (r Registry) Worktree(parent, name string) (*core.Worktree, error) {
	if parent == "" || name == "" {
		return nil, nil
	}

	contents, err := os.ReadFile(filepath.Join(r.home(), parent, name+".json"))
	if err != nil {
		return nil, nil
	}

	var record registration
	if err := json.Unmarshal(contents, &record); err != nil {
		return nil, nil
	}

	return &core.Worktree{
		Name:    name,
		Branch:  record.Branch,
		Parent:  parent,
		Profile: record.Profile,
		Project: record.Project,
		Domain:  record.Domain,
		Path:    record.Path,

		// Only while they are the same dependencies: equal locks mean identical trees, and a
		// different lock means the branch changed them and sharing would be a lie. It is decided
		// once, when the worktree is created, and written down
		SharedVendor: record.Vendor == "shared",
	}, nil
}

// Overlay is the compose file that carries a branch environment's profile and routing.
func (r Registry) Overlay(parent, name string) string {
	return filepath.Join(r.home(), parent, name+".yml")
}

func (r Registry) home() string {
	if r.Dir != "" {
		return r.Dir
	}

	if dir := os.Getenv("HM_WORKTREE_DIR"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".hm", "worktrees")
}

// Worktrees is every branch environment of a project.
func (r Registry) Worktrees(parent string) ([]core.Worktree, error) {
	entries, err := os.ReadDir(filepath.Join(r.home(), parent))
	if err != nil {
		return []core.Worktree{}, nil
	}

	worktrees := []core.Worktree{}

	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || name == entry.Name() {
			continue
		}

		worktree, err := r.Worktree(parent, name)
		if err != nil || worktree == nil {
			continue
		}

		worktrees = append(worktrees, *worktree)
	}

	sort.Slice(worktrees, func(a, b int) bool { return worktrees[a].Name < worktrees[b].Name })

	return worktrees, nil
}

// Forget removes a registration and the overlay beside it.
//
// Both, because they are one fact in two files: an overlay with no registration is a compose file
// nothing loads, and a registration with no overlay is an environment that cannot be built.
func (r Registry) Forget(parent, name string) error {
	if err := os.Remove(filepath.Join(r.home(), parent, name+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(r.Overlay(parent, name)); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Only when it is empty, which is what tells the last worktree of a project from the others
	os.Remove(filepath.Join(r.home(), parent)) //nolint:errcheck

	return nil
}

// Save writes a registration.
//
// Written beside and moved into place: a half-written registration is a branch environment that
// resolves to nothing, and this runs while other agents are reading the same directory.
func (r Registry) Save(parent string, worktree core.Worktree) error {
	directory := filepath.Join(r.home(), parent)

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	vendor := "own"
	if worktree.SharedVendor {
		vendor = "shared"
	}

	document, err := json.MarshalIndent(registration{
		Parent:  parent,
		Path:    worktree.Path,
		Branch:  worktree.Branch,
		Profile: worktree.Profile,
		Domain:  worktree.Domain,
		Project: worktree.Project,
		Vendor:  vendor,
		Created: time.Now().Format("2006-01-02 15:04"),
	}, "", "  ")
	if err != nil {
		return err
	}

	file := filepath.Join(directory, worktree.Name+".json")
	temporary := file + ".tmp"

	if err := os.WriteFile(temporary, append(document, '\n'), 0o644); err != nil { //nolint:gosec
		return err
	}

	return os.Rename(temporary, file)
}

// WriteOverlay writes the compose file that turns the project's configuration into this branch's
// environment.
func (r Registry) WriteOverlay(parent, name, contents string) error {
	if err := os.MkdirAll(filepath.Join(r.home(), parent), 0o755); err != nil {
		return err
	}

	return os.WriteFile(r.Overlay(parent, name), []byte(contents), 0o644) //nolint:gosec
}
