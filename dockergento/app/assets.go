package app

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// asset is one of the files the tool ships with, or empty when it is not there — the same answer
// a missing file on disk gives through ports.FS.Read. Nil assets have no files.
func asset(assets fs.FS, name string) string {
	if assets == nil {
		return ""
	}

	contents, err := fs.ReadFile(assets, name)
	if err != nil {
		return ""
	}

	return string(contents)
}

// extractAssets writes the directories named in dirs, from the files the tool ships with, under
// target on disk: what a step that works with paths — copying, digesting, registering — needs of
// files that may only exist inside the binary. A directory the assets do not have is skipped.
func extractAssets(assets fs.FS, target string, dirs ...string) error {
	if assets == nil {
		return nil
	}

	for _, dir := range dirs {
		if info, err := fs.Stat(assets, dir); err != nil || !info.IsDir() {
			continue
		}

		err := fs.WalkDir(assets, dir, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			destination := filepath.Join(target, filepath.FromSlash(name))
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}

			contents, err := fs.ReadFile(assets, name)
			if err != nil {
				return err
			}

			// What is executable stays so; an embedded file has no write bit, and the copy needs one
			mode := fs.FileMode(0o644)
			if info, err := entry.Info(); err == nil {
				mode |= info.Mode().Perm()
			}

			return os.WriteFile(destination, contents, mode)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// assetDir is whether the files the tool ships with have a directory of that name.
func assetDir(assets fs.FS, name string) bool {
	if assets == nil {
		return false
	}

	info, err := fs.Stat(assets, path.Clean(name))

	return err == nil && info.IsDir()
}
