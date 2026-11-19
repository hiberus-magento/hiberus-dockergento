package core

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sort"
)

// DigestPath is the digest of a file, or of a directory's contents, as console/helpers/digest.sh's
// hm_digest_path computes it — the checksum ai-pull records and ai-doctor compares, so the two
// implementations have to agree on every byte that goes into it.
//
// A file is its contents. A directory is every regular file under it, in byte order of its path
// relative to the directory (`find | LC_ALL=C sort`), each as that path and a newline followed by
// the file's contents: the names go in alongside the contents, so two skills with the same text
// under different file names are not the same skill, and a file appearing or disappearing changes
// the result. Like find, it neither follows a symbolic link nor counts one as a file, and a
// directory reached through a link has nothing in it. Nothing there is the empty string.
func DigestPath(fsys fs.FS, name string) string {
	info, err := fs.Lstat(fsys, name)
	if err != nil {
		return ""
	}

	linked := info.Mode()&fs.ModeSymlink != 0
	if linked {
		if info, err = fs.Stat(fsys, name); err != nil {
			return ""
		}
	}

	hash := sha256.New()

	switch {
	case info.IsDir() && linked:
		// `find link -type f` does not descend into it: the digest of nothing
	case info.IsDir():
		for _, file := range regularFiles(fsys, name) {
			hash.Write([]byte(file + "\n"))

			if contents, err := fs.ReadFile(fsys, name+"/"+file); err == nil {
				hash.Write(contents)
			}
		}
	case info.Mode().IsRegular():
		contents, err := fs.ReadFile(fsys, name)
		if err != nil {
			return ""
		}

		hash.Write(contents)
	default:
		return ""
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// regularFiles is every regular file under a directory, relative to it and in byte order.
func regularFiles(fsys fs.FS, root string) []string {
	var files []string

	_ = fs.WalkDir(fsys, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// What find cannot read it leaves out, and goes on
			return nil
		}

		if entry.Type().IsRegular() {
			files = append(files, path[len(root)+1:])
		}

		return nil
	})

	sort.Strings(files)

	return files
}
