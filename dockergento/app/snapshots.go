package app

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Snapshots are the named copies of a project's database.
//
// `mysqldump` and an import already move a database in and out of a file. What was missing was
// management: somewhere for the copies to live, a name to call them by and a list to see them in.
// Without that, saving before a risky operation depends on somebody thinking of it, which is why
// it does not happen.
type Snapshots struct {
	Database Database
	State    ports.DataState

	// Dir is where every project's copies live on this machine.
	Dir string

	// Version is the Magento this environment runs, recorded in the file's header so that a copy
	// found a year later says what it came from.
	Version string

	Progress func(label string) func(ok bool, note string)
	Ask      func(question, suggestion string) (string, error)

	// Errors is where the dumper's own complaints go.
	Errors io.Writer

	// Binary is the name the tool was invoked as, for the hints.
	Binary string
}

// The dumper, resolved inside the container for the same reason the client is: MariaDB 11 dropped
// the legacy names and 10.x does not have the new ones.
//
// --single-transaction takes the copy from a consistent point without locking the tables, so the
// project keeps working while it runs. Routines, triggers and events are included: a copy that
// restores a Magento without them is not a copy of that Magento.
const dumper = `dump=$(command -v mariadb-dump || command -v mysqldump); ` +
	`"$dump" --single-transaction --quick --no-tablespaces --routines --triggers --events ` +
	`-u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"`

// Take saves the database as a named copy.
func (s Snapshots) Take(project core.Project, name string, forced bool) (core.Snapshot, error) {
	if name == "" {
		name = time.Now().Format("2006-01-02-150405")
	}

	if !core.ValidSnapshotName(name) {
		return core.Snapshot{}, core.Refusal{
			Kind:    "invalid_name",
			Code:    2,
			Message: fmt.Sprintf("'%s' cannot be used as a snapshot name", name),
			Hint:    "Letters, digits, dots, dashes and underscores, not starting with a dot or a dash",
		}
	}

	target := s.path(project, name)

	if _, err := os.Stat(target); err == nil && !forced {
		return core.Snapshot{}, core.Refusal{
			Kind:    "snapshot_exists",
			Code:    2,
			Message: fmt.Sprintf("This project already has a snapshot called '%s'", name),
			Hint:    fmt.Sprintf("%s db snapshot --name=%s --force", s.Binary, name),
		}
	}

	if _, err := s.Database.Ready(project); err != nil {
		return core.Snapshot{}, err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return core.Snapshot{}, err
	}

	takenAt := time.Now().Format("2006-01-02 15:04:05")

	done := s.begin(fmt.Sprintf("Saving the database as '%s'...", name))

	// Written beside the copy and renamed only once it is complete: an interrupted dump must not
	// look like a usable snapshot, and it is the moment somebody most needs one to be usable
	status, err := s.write(project, target+".partial", name, takenAt)

	done(err == nil && status == 0, "")

	if err != nil {
		os.Remove(target + ".partial") //nolint:errcheck

		return core.Snapshot{}, err
	}

	if status != 0 {
		os.Remove(target + ".partial") //nolint:errcheck

		return core.Snapshot{}, core.Refusal{
			Kind:    "snapshot_failed",
			Code:    1,
			Message: "The database could not be copied",
			Hint:    fmt.Sprintf("%s logs db", s.Binary),
		}
	}

	if err := os.Rename(target+".partial", target); err != nil {
		return core.Snapshot{}, err
	}

	return s.describe(target, name), nil
}

// write runs the dump into a compressed file, with a header saying what it is.
func (s Snapshots) write(project core.Project, target, name, takenAt string) (int, error) {
	file, err := os.Create(target) //nolint:gosec
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Six, as the shell implementation used: the level where a Magento dump stops getting much
	// smaller and starts taking noticeably longer
	compressed, err := gzip.NewWriterLevel(file, 6)
	if err != nil {
		return 0, err
	}

	version := s.Version
	if version == "" {
		version = "unknown"
	}

	if _, err := fmt.Fprintf(compressed, "-- hm snapshot: %s\n-- taken: %s\n-- magento: %s\n",
		name, takenAt, version); err != nil {
		return 0, err
	}

	status, err := s.Database.Capture(project, dumper, compressed, s.Errors)
	if err != nil {
		return status, err
	}

	return status, compressed.Close()
}

// List is every copy of this project, newest name last, as they are read from disk.
func (s Snapshots) List(project core.Project) ([]core.Snapshot, error) {
	entries, err := os.ReadDir(s.root(project))
	if err != nil {
		// A project with no copies is not an error, and neither is a machine that has never
		// taken one
		return []core.Snapshot{}, nil //nolint:nilerr
	}

	found := make([]core.Snapshot, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql.gz") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".sql.gz")
		found = append(found, s.describe(filepath.Join(s.root(project), entry.Name()), name))
	}

	sort.Slice(found, func(a, b int) bool { return found[a].Name < found[b].Name })

	return found, nil
}

// Restore replaces the database with a copy, and answers whether it happened.
//
// The confirmation is the project's name typed out rather than a letter: a blind `y` is a reflex,
// typing the name means the sentence was read. It is the only thing here that destroys data.
func (s Snapshots) Restore(project core.Project, name string, interactive bool) (bool, error) {
	source, err := s.find(project, name, "restored")
	if err != nil {
		return false, err
	}

	if _, err := s.Database.Ready(project); err != nil {
		return false, err
	}

	if interactive && s.Ask != nil {
		answer, err := s.Ask(fmt.Sprintf(
			"This replaces the database of '%s' with '%s'. Everything in it since that snapshot "+
				"will be lost.\nType the project name to confirm", project.Name, name), "")
		if err != nil {
			return false, err
		}

		if strings.TrimSpace(answer) != project.Name {
			return false, nil
		}
	}

	file, err := os.Open(source) //nolint:gosec
	if err != nil {
		return false, err
	}
	defer file.Close()

	contents, err := gzip.NewReader(file)
	if err != nil {
		return false, core.Refusal{
			Kind:    "unreadable_snapshot",
			Code:    1,
			Message: fmt.Sprintf("The snapshot '%s' cannot be read", name),
			Hint:    fmt.Sprintf("%s db list", s.Binary),
		}
	}
	defer contents.Close()

	done := s.begin(fmt.Sprintf("Restoring '%s'...", name))

	//
	// Emptied first. Restoring over a database that kept living would leave whatever was created
	// afterwards in place, and the result would not be the snapshot but a mixture of the two.
	//
	if _, err := s.Database.Capture(project, empty, s.Errors, s.Errors); err != nil {
		done(false, "")

		return false, err
	}

	status, err := s.Database.Feed(project, contents, s.Errors)

	done(err == nil && status == 0, "")

	if err != nil {
		return false, err
	}

	if status != 0 {
		return false, core.Refusal{
			Kind:    "restore_failed",
			Code:    1,
			Message: fmt.Sprintf("The snapshot '%s' could not be restored", name),
			Hint:    fmt.Sprintf("%s logs db", s.Binary),
		}
	}

	// Whatever that copy holds, nobody anonymised it after the fact
	return true, s.State.Clear(project.Name)
}

// empty drops the database and creates it again, which is what makes a restore a replacement.
const empty = `client=$(command -v mariadb || command -v mysql); ` +
	`"$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" ` +
	"-e \"DROP DATABASE IF EXISTS \\`$MYSQL_DATABASE\\`; CREATE DATABASE \\`$MYSQL_DATABASE\\`;\""

// Remove deletes one copy by name.
func (s Snapshots) Remove(project core.Project, name string) error {
	target, err := s.find(project, name, "removed")
	if err != nil {
		return err
	}

	return os.Remove(target)
}

// Cleared is what a clear did, or would do.
type Cleared struct {
	Removed int    `json:"removed"`
	Freed   string `json:"freed"`
	Scope   string `json:"scope"`

	// Found is how many there were to delete, which is what tells "there is nothing here" from
	// "somebody said no". Both delete nothing, and they are not the same answer.
	Found int `json:"-"`
}

// Clear deletes every copy of this project, or of every project on the machine.
//
// `remove` deletes one by name; this is for reclaiming the space, which is the other reason to
// delete. It asks harder, because it is the only thing here that can destroy copies belonging to
// projects you are not standing in.
func (s Snapshots) Clear(project core.Project, all, interactive bool) (Cleared, error) {
	root := s.root(project)
	scope := "project"
	confirmation := project.Name
	subject := project.Name

	if all {
		root = s.Dir
		scope = "all"
		confirmation = "all"
		subject = "every project on this machine"
	}

	targets, bytes := s.collect(root)

	if len(targets) == 0 {
		return Cleared{Freed: "0B", Scope: scope}, nil
	}

	total := core.SnapshotSize(bytes)

	if interactive && s.Ask != nil {
		listed := make([]string, 0, len(targets))
		for _, target := range targets {
			listed = append(listed, "  "+strings.TrimPrefix(target, s.Dir+string(filepath.Separator)))
		}

		// What is being destroyed is named in the answer, so a reflex cannot do it
		answer, err := s.Ask(fmt.Sprintf(
			"This deletes %d snapshot(s) of %s, freeing %s. There is no undo, and they are the "+
				"only copies.\n\n%s\n\nType '%s' to confirm",
			len(targets), subject, total, strings.Join(listed, "\n"), confirmation), "")
		if err != nil {
			return Cleared{}, err
		}

		if strings.TrimSpace(answer) != confirmation {
			return Cleared{Freed: "0B", Scope: scope, Found: len(targets)}, nil
		}
	}

	for _, target := range targets {
		if err := os.Remove(target); err != nil {
			return Cleared{}, err
		}
	}

	s.prune()

	return Cleared{Removed: len(targets), Freed: total, Scope: scope, Found: len(targets)}, nil
}

// collect is every copy under a directory, and what they take up together.
func (s Snapshots) collect(root string) ([]string, int64) {
	found := []string{}
	total := int64(0)

	filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error { //nolint:errcheck
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".sql.gz") {
			return nil //nolint:nilerr
		}

		found = append(found, path)
		total += usage(path)

		return nil
	})

	sort.Strings(found)

	return found, total
}

// prune leaves no empty project directories behind.
func (s Snapshots) prune() {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			os.Remove(filepath.Join(s.Dir, entry.Name())) //nolint:errcheck
		}
	}
}

// find is the file a name refers to, or the refusal that says it is not there.
//
// The verb is the caller's because the question is: somebody who typed `db remove` is not being
// asked which one to restore.
func (s Snapshots) find(project core.Project, name, verb string) (string, error) {
	if name == "" {
		return "", core.Refusal{
			Kind:    "missing_name",
			Code:    2,
			Message: fmt.Sprintf("Which snapshot should be %s?", verb),
			Hint:    fmt.Sprintf("%s db list", s.Binary),
		}
	}

	if !core.ValidSnapshotName(name) {
		return "", core.Refusal{
			Kind:    "invalid_name",
			Code:    2,
			Message: fmt.Sprintf("'%s' cannot be used as a snapshot name", name),
			Hint:    "Letters, digits, dots, dashes and underscores, not starting with a dot or a dash",
		}
	}

	target := s.path(project, name)

	if _, err := os.Stat(target); err != nil {
		return "", core.Refusal{
			Kind:    "unknown_snapshot",
			Code:    2,
			Message: fmt.Sprintf("This project has no snapshot called '%s'", name),
			Hint:    fmt.Sprintf("%s db list", s.Binary),
		}
	}

	return target, nil
}

func (s Snapshots) root(project core.Project) string { return filepath.Join(s.Dir, project.Name) }

func (s Snapshots) path(project core.Project, name string) string {
	return filepath.Join(s.root(project), name+".sql.gz")
}

// describe is what a copy looks like from outside: its size on disk and when it was written.
func (s Snapshots) describe(path, name string) core.Snapshot {
	bytes := usage(path)
	taken := ""

	if info, err := os.Stat(path); err == nil {
		taken = info.ModTime().Format("2006-01-02 15:04")
	}

	return core.Snapshot{
		Name:    name,
		Path:    path,
		Size:    core.SnapshotSize(bytes),
		Bytes:   bytes,
		TakenAt: taken,
	}
}

func (s Snapshots) begin(label string) func(bool, string) {
	if s.Progress == nil {
		return func(bool, string) {}
	}

	return s.Progress(label)
}
