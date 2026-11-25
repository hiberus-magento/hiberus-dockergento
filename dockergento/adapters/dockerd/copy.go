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

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
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

var _ ports.MeteredFileTransfer = Files{}

// Into copies a file or a directory from this machine into a container.
//
// The target is a directory, as it is for `docker cp`: a file lands in it under its own name, and
// a directory lands in it as itself.
func (f Files) Into(id, source, target string) error {
	return f.IntoMetered(id, source, target, nil)
}

// IntoMetered is Into, telling meter how far it has got; a nil meter is Into.
func (Files) IntoMetered(id, source, target string, meter ports.CopyMeter) error {
	docker, err := connect()
	if err != nil {
		return err
	}
	defer docker.Close()

	read, write := io.Pipe()

	go func() {
		write.CloseWithError(pack(write, source, meter)) //nolint:errcheck
	}()

	defer read.Close()

	return docker.CopyToContainer(context.Background(), id, target, read,
		container.CopyToContainerOptions{})
}

// From copies a path out of a container onto this machine.
func (f Files) From(id, source, target string) error {
	return f.FromMetered(id, source, target, nil)
}

// FromMetered is From, telling meter how far it has got; a nil meter is From.
func (Files) FromMetered(id, source, target string, meter ports.CopyMeter) error {
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

	return unpack(stream, target, filepath.Base(strings.TrimSuffix(source, "/.")), meter)
}

// pack writes a path into a tar stream, as the daemon expects to receive it.
func pack(out io.Writer, source string, meter ports.CopyMeter) error {
	source = filepath.Clean(source)

	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	archive := tar.NewWriter(out)
	defer archive.Close()

	count := newTally(meter)

	// A file is sent under its own name; a directory is sent as itself with everything under it,
	// which is what makes the two behave the way `docker cp` behaves
	base := filepath.Dir(source)

	if !info.IsDir() {
		return add(archive, source, filepath.Base(source), count)
	}

	return filepath.Walk(source, func(path string, entry os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}

		return add(archive, path, name, count)
	})
}

// add writes one entry, following what it finds rather than what it was told: a symbolic link
// carried across as a link is a link to somewhere that does not exist on the other side.
func add(archive *tar.Writer, path, name string, count *tally) error {
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

	count.entry(header)

	if info.IsDir() {
		return nil
	}

	file, err := os.Open(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(archive, count.through(file))

	return err
}

// unpack writes a tar stream from the daemon onto this machine.
//
// The stream carries the copied path as its top-level entry. When the target is where that entry
// should land, its own name is stripped: `docker cp container:/x/y ./y` writes `./y`, not `./y/y`.
func unpack(stream io.Reader, target, top string, meter ports.CopyMeter) error {
	archive := tar.NewReader(stream)
	strip := filepath.Base(target) == top
	count := newTally(meter)

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

		count.entry(header)

		if err := restore(count.through(archive), header, path); err != nil {
			return err
		}
	}
}

func restore(archive io.Reader, header *tar.Header, path string) error {
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

// tally counts what a copy carries, the contents of its files and how many there are, and tells
// the meter as it goes. A nil tally counts nothing, which is a copy without a meter.
type tally struct {
	meter        ports.CopyMeter
	bytes, files int64
}

func newTally(meter ports.CopyMeter) *tally {
	if meter == nil {
		return nil
	}

	return &tally{meter: meter}
}

// entry counts one entry of the archive; only a regular file is a file, a directory or a link is
// not what anybody is counting.
func (t *tally) entry(header *tar.Header) {
	if t == nil || header.Typeflag != tar.TypeReg {
		return
	}

	t.files++
	t.meter(t.bytes, t.files)
}

// through is what a file's contents are read through to be counted.
func (t *tally) through(contents io.Reader) io.Reader {
	if t == nil {
		return contents
	}

	return io.TeeReader(contents, t)
}

func (t *tally) Write(block []byte) (int, error) {
	t.bytes += int64(len(block))
	t.meter(t.bytes, t.files)

	return len(block), nil
}
