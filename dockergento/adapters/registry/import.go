package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The state the shell implementation wrote, brought across.
//
// It has to be idempotent and it has to lose nothing: this runs on machines with worktrees people
// are working in, and an import that dropped one would leave an environment nothing can find and
// nothing will clean up.
//

type registration struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Profile string `json:"profile"`
	Domain  string `json:"domain"`
	Project string `json:"project"`
	Vendor  string `json:"vendor"`
}

type anonymisation struct {
	AnonymisedAt string `json:"anonymised_at"`
}

// Imported is what an import found and brought in.
type Imported struct {
	Worktrees int `json:"worktrees"`
	States    int `json:"states"`
}

// Import brings in what the shell implementation recorded in ~/.hm.
//
// It is safe to run again: a registration already here is updated rather than duplicated, and a
// file that cannot be read is skipped rather than failing the whole import — one unreadable record
// is not a reason to leave the other twenty behind.
func (s *Store) Import(worktreeDir, stateDir string) (Imported, error) {
	brought := Imported{}

	projects, err := os.ReadDir(worktreeDir)
	if err == nil {
		for _, project := range projects {
			if !project.IsDir() {
				continue
			}

			brought.Worktrees += s.importWorktrees(filepath.Join(worktreeDir, project.Name()), project.Name())
		}
	}

	records, err := os.ReadDir(stateDir)
	if err == nil {
		for _, record := range records {
			name := strings.TrimSuffix(record.Name(), ".json")
			if record.IsDir() || name == record.Name() {
				continue
			}

			var state anonymisation
			if !read(filepath.Join(stateDir, record.Name()), &state) || state.AnonymisedAt == "" {
				continue
			}

			if err := s.RecordAnonymisation(name, state.AnonymisedAt); err == nil {
				brought.States++
			}
		}
	}

	return brought, nil
}

func (s *Store) importWorktrees(dir, project string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	brought := 0

	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".json")
		if entry.IsDir() || name == entry.Name() {
			continue
		}

		var record registration
		if !read(filepath.Join(dir, entry.Name()), &record) {
			continue
		}

		// A registration with no environment name is one no command could have used anyway: the
		// name is what its containers answer to
		if record.Project == "" {
			continue
		}

		worktree := core.Worktree{
			Name:         name,
			Branch:       record.Branch,
			Parent:       project,
			Profile:      record.Profile,
			Project:      record.Project,
			Domain:       record.Domain,
			Path:         record.Path,
			SharedVendor: record.Vendor == "shared",
		}

		if err := s.Register(core.Project{Name: project}, worktree); err == nil {
			brought++
		}
	}

	return brought
}

func read(path string, into any) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return json.Unmarshal(contents, into) == nil
}
