package dockerd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
)

//
// Moving files between this machine and a container.
//
// Through the daemon's own copy, which takes and gives a tar stream: the same thing `docker cp`
// does, without the process. What it is for is the one case macOS makes necessary — the code lives
// in a volume for speed, so what is written on one side has to be carried to the other.
//

// Files copies between the host and a container.
type Files struct{}

// Into copies a file or a directory from this machine into a container.
//
// The target is a directory, as it is for `docker cp`: a file lands in it under its own name, and
// a directory lands in it as itself.
func (Files) Into(id, source, target string) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	read, write := io.Pipe()

	go func() {
		write.CloseWithError(pack(write, source)) //nolint:errcheck
	}()

	defer read.Close()

	return docker.CopyToContainer(context.Background(), id, target, read,
		container.CopyToContainerOptions{})
}

// From copies a path out of a container onto this machine.
func (Files) From(id, source, target string) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	stream, _, err := docker.CopyFromContainer(context.Background(), id, source)
	if err != nil {
		return err
	}
	defer stream.Close()

	return unpack(stream, target, filepath.Base(strings.TrimSuffix(source, "/.")))
}

// pack writes a path into a tar stream, as the daemon expects to receive it.
func pack(out io.Writer, source string) error {
	source = filepath.Clean(source)

	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	archive := tar.NewWriter(out)
	defer archive.Close()

	// A file is sent under its own name; a directory is sent as itself with everything under it,
	// which is what makes the two behave the way `docker cp` behaves
	base := filepath.Dir(source)

	if !info.IsDir() {
		return add(archive, source, filepath.Base(source))
	}

	return filepath.Walk(source, func(path string, entry os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}

		return add(archive, path, name)
	})
}

// add writes one entry, following what it finds rather than what it was told: a symbolic link
// carried across as a link is a link to somewhere that does not exist on the other side.
func add(archive *tar.Writer, path, name string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if info, err = os.Stat(path); err != nil {
			// A link to nothing is skipped rather than failing the copy: it is somebody's
			// leftover, and the rest of the tree is what was asked for
			return nil //nolint:nilerr
		}
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	header.Name = filepath.ToSlash(name)

	if err := archive.WriteHeader(header); err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	file, err := os.Open(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(archive, file)

	return err
}

// unpack writes a tar stream from the daemon onto this machine.
//
// The stream carries the copied path as its top-level entry. When the target is where that entry
// should land, its own name is stripped: `docker cp container:/x/y ./y` writes `./y`, not `./y/y`.
func unpack(stream io.Reader, target, top string) error {
	archive := tar.NewReader(stream)
	strip := filepath.Base(target) == top

	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		name := header.Name
		if strip {
			name = strings.TrimPrefix(strings.TrimPrefix(name, top), "/")
		}

		path := filepath.Join(target, filepath.Clean("/"+name))

		// Nothing is written outside the target, whatever the stream says its entries are called
		if !strings.HasPrefix(path, filepath.Clean(target)) {
			return fmt.Errorf("refusing to write outside %s: %s", target, header.Name)
		}

		if err := restore(archive, header, path); err != nil {
			return err
		}
	}
}

func restore(archive *tar.Reader, header *tar.Header, path string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(path, os.FileMode(header.Mode))
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(file, archive) //nolint:gosec

		return err
	case tar.TypeSymlink:
		os.Remove(path) //nolint:errcheck

		return os.Symlink(header.Linkname, path)
	}

	// Anything else — a device, a socket — is not something a project's code contains, and
	// recreating one is not something this should be doing
	return nil
}
