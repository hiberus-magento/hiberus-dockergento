// Package registry is what this machine knows about the projects on it.
//
// One SQLite file instead of a directory of small JSON ones, for two reasons that the JSON version
// could not give: a slot has to be handed out atomically — two agents registering a worktree at
// the same moment must not receive the same schema — and removing a worktree has to take its
// allocation with it in the same breath. A lock around a directory can be made to work; a
// transaction already is one.
//
// What it does not hold is observed state. Which containers exist is asked of Docker, because a
// registry goes on claiming an environment exists after somebody removed it by hand. This holds
// what somebody decided, which is the part Docker cannot answer.
package registry

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Pure Go, no cgo: the Linux binaries are built with cgo off and cross-compiled, and a driver
	// that needed a C compiler would end that.
	_ "modernc.org/sqlite"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//go:embed schema.sql
var schema string

// Version is what the file says it is, so that a binary older than the file can say so instead of
// reading it wrong.
const Version = 1

// Store is the registry.
type Store struct {
	db *sql.DB
}

// Open opens the registry, creating it if it is not there.
//
// The pragmas are the point of the exercise. Write-ahead logging lets a reader and a writer work
// at once, which is what several agents on one machine are; the busy timeout turns "the database
// is locked" — the failure everybody meets with SQLite — into a wait.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	//
	// `_txlock=immediate` is not a detail. In write-ahead logging a transaction that begins by
	// reading and then writes has to be rejected outright when another writer committed in
	// between — the busy timeout does not cover it, because there is nothing to wait for: the
	// snapshot it read is already stale. Taking the write lock up front turns that into a wait,
	// which is what handing out a slot to two agents at once needs.
	//
	db, err := sql.Open("sqlite", "file:"+path+
		"?_txlock=immediate"+
		"&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()

		return nil, fmt.Errorf("the registry could not be prepared: %w", err)
	}

	if _, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value WHERE CAST(value AS INTEGER) < ?`,
		Version, Version); err != nil {
		db.Close()

		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SaveProject records a project, or updates where it is and what topology it uses.
//
// An empty root does not blank one already recorded: the registrations imported from the old JSON
// files know where a worktree is but not where its project is, and learning it later is better
// than forgetting it now.
func (s *Store) SaveProject(project core.Project) error {
	topology := string(project.Topology)
	if topology == "" {
		topology = string(core.Classic)
	}

	_, err := s.db.Exec(
		`INSERT INTO projects (name, root, topology, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   root = CASE WHEN excluded.root != '' THEN excluded.root ELSE root END,
		   topology = excluded.topology`,
		project.Name, project.Root, topology, now())

	return err
}

// Projects is every project this machine knows about.
func (s *Store) Projects() ([]core.Project, error) {
	rows, err := s.db.Query(`SELECT name, root, topology FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []core.Project{}

	for rows.Next() {
		var project core.Project
		var topology string

		if err := rows.Scan(&project.Name, &project.Root, &topology); err != nil {
			return nil, err
		}

		project.Topology = core.Topology(topology)
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// Worktree returns the registration of a branch environment, or nil when there is none.
//
// Nil is not an error and is the answer that matters most: it is the difference between a worktree
// with an environment of its own and a worktree borrowing the main one's identity.
func (s *Store) Worktree(project, name string) (*core.Worktree, error) {
	if project == "" || name == "" {
		return nil, nil
	}

	worktree, err := s.scanWorktree(s.db.QueryRow(
		`SELECT name, path, branch, profile, domain, environment, vendor
		   FROM worktrees WHERE project = ? AND name = ?`, project, name), project)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return worktree, err
}

// Worktrees is every branch environment of a project.
func (s *Store) Worktrees(project string) ([]core.Worktree, error) {
	rows, err := s.db.Query(
		`SELECT name, path, branch, profile, domain, environment, vendor
		   FROM worktrees WHERE project = ? ORDER BY name`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	worktrees := []core.Worktree{}

	for rows.Next() {
		worktree, err := s.scanWorktree(rows, project)
		if err != nil {
			return nil, err
		}

		worktrees = append(worktrees, *worktree)
	}

	return worktrees, rows.Err()
}

type scanner interface{ Scan(...any) error }

func (s *Store) scanWorktree(row scanner, project string) (*core.Worktree, error) {
	var worktree core.Worktree
	var vendor string

	if err := row.Scan(&worktree.Name, &worktree.Path, &worktree.Branch,
		&worktree.Profile, &worktree.Domain, &worktree.Project, &vendor); err != nil {
		return nil, err
	}

	worktree.Parent = project
	worktree.SharedVendor = vendor == "shared"

	return &worktree, nil
}

// Register records a branch environment, and the project it belongs to if it was not there.
func (s *Store) Register(parent core.Project, worktree core.Worktree) error {
	if err := s.SaveProject(parent); err != nil {
		return err
	}

	vendor := "own"
	if worktree.SharedVendor {
		vendor = "shared"
	}

	_, err := s.db.Exec(
		`INSERT INTO worktrees (project, name, path, branch, profile, domain, environment, vendor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project, name) DO UPDATE SET
		   path = excluded.path, branch = excluded.branch, profile = excluded.profile,
		   domain = excluded.domain, environment = excluded.environment, vendor = excluded.vendor`,
		parent.Name, worktree.Name, worktree.Path, worktree.Branch, worktree.Profile,
		worktree.Domain, worktree.Project, vendor, now())

	return err
}

// Forget removes a branch environment, and its allocation with it.
//
// In one statement, because the foreign key cascades: a worktree gone and its slot still taken is
// how a project runs out of schemas it is not using.
func (s *Store) Forget(project, name string) error {
	_, err := s.db.Exec(`DELETE FROM worktrees WHERE project = ? AND name = ?`, project, name)

	return err
}

//
// Anonymisation: whether this copy of the data has been anonymised, and when.
//
// Unknown is the honest answer for an environment nobody has touched, and it is never treated as
// safe. What makes it worth recording is that it expires — everything that replaces the contents
// of a database clears it, because a reassuring "yes" left over from before an import is worse
// than no record at all.
//

// Anonymisation reports "yes" or "unknown", and when.
func (s *Store) Anonymisation(environment string) (string, string) {
	var at string

	err := s.db.QueryRow(`SELECT anonymised_at FROM data_state WHERE environment = ?`,
		environment).Scan(&at)
	if err != nil || at == "" {
		return "unknown", ""
	}

	return "yes", at
}

// RecordAnonymisation writes it down.
func (s *Store) RecordAnonymisation(environment, at string) error {
	_, err := s.db.Exec(
		`INSERT INTO data_state (environment, anonymised_at) VALUES (?, ?)
		 ON CONFLICT(environment) DO UPDATE SET anonymised_at = excluded.anonymised_at`,
		environment, at)

	return err
}

// ClearAnonymisation is called by everything that replaces the database. Whatever it brought in,
// nobody anonymised it.
func (s *Store) ClearAnonymisation(environment string) error {
	_, err := s.db.Exec(`DELETE FROM data_state WHERE environment = ?`, environment)

	return err
}

//
// Slots, which is the whole reason this is a database.
//

// Allocate gives a worktree the lowest free slot, and everything derived from it.
//
// In a transaction, because two agents creating a worktree at the same moment reading "the lowest
// free slot" and then both writing it is not a rare race: it is what happens the first time
// somebody runs two of them in parallel, and the symptom is two branches sharing a database.
func (s *Store) Allocate(project, worktree string) (core.Allocation, error) {
	transaction, err := s.db.Begin()
	if err != nil {
		return core.Allocation{}, err
	}
	defer transaction.Rollback() //nolint:errcheck

	var existing int
	err = transaction.QueryRow(`SELECT slot FROM allocations WHERE project = ? AND worktree = ?`,
		project, worktree).Scan(&existing)
	if err == nil {
		return core.AllocationFor(worktree, existing), transaction.Commit()
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return core.Allocation{}, err
	}

	// The lowest number not already taken, so that slots freed by a removed worktree come back
	var slot int
	err = transaction.QueryRow(
		`SELECT COALESCE(MIN(candidate), 0) FROM (
		   SELECT 0 AS candidate WHERE NOT EXISTS (SELECT 1 FROM allocations WHERE project = ?1 AND slot = 0)
		   UNION ALL
		   SELECT slot + 1 FROM allocations WHERE project = ?1
		     AND slot + 1 NOT IN (SELECT slot FROM allocations WHERE project = ?1)
		 )`, project).Scan(&slot)
	if err != nil {
		return core.Allocation{}, err
	}

	if slot >= core.MaxSlots {
		return core.Allocation{}, core.ErrNoSlots{Project: project}
	}

	allocation := core.AllocationFor(worktree, slot)

	if _, err := transaction.Exec(
		`INSERT INTO allocations
		   (project, worktree, slot, schema_name, index_prefix, cache_db, page_cache_db, session_db, vhost)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project, worktree, allocation.Slot, allocation.Schema, allocation.IndexPrefix,
		allocation.CacheDB, allocation.PageCacheDB, allocation.SessionDB, allocation.VHost); err != nil {
		return core.Allocation{}, err
	}

	return allocation, transaction.Commit()
}

// Allocation returns what a worktree was given, or nil when it has nothing.
func (s *Store) Allocation(project, worktree string) (*core.Allocation, error) {
	var slot int

	err := s.db.QueryRow(`SELECT slot FROM allocations WHERE project = ? AND worktree = ?`,
		project, worktree).Scan(&slot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	allocation := core.AllocationFor(worktree, slot)

	return &allocation, nil
}

// Release gives a slot back.
func (s *Store) Release(project, worktree string) error {
	_, err := s.db.Exec(`DELETE FROM allocations WHERE project = ? AND worktree = ?`, project, worktree)

	return err
}

func now() string { return time.Now().Format("2006-01-02 15:04:05") }
