// Package osfs is the filesystem, and the git questions that need one.
package osfs

import (
	"os"
	"os/exec"
	"strings"
)

// FS answers whether paths are there.
type FS struct{}

// IsDir reports whether the path is a directory that exists.
func (FS) IsDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// Exists reports whether the path is there at all, directory or not.
func (FS) Exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// Read returns a file's contents, or an empty string when it cannot be read. A file that is not
// there is not an error for any of the callers here: it is an answer.
func (FS) Read(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(contents)
}

// Branches reports the branch checked out in a directory.
type Branches struct{}

// Branch returns the current branch, or an empty string when the directory is not a repository or
// git is not there. Not knowing the branch is not an error: it is a column that stays empty.
func (Branches) Branch(dir string) string {
	command := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	command.Dir = dir

	output, err := command.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}
