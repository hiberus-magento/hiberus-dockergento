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

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Registry is ~/.hm/worktrees.
type Registry struct {
	// Dir overrides where the registrations live. Empty means the default.
	Dir string
}

type registration struct {
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
