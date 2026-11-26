package app

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Transfer moves a project's files between this machine and its php container.
//
// It exists because of macOS. There the code is copied into a volume rather than mounted, which is
// what makes PHP fast enough to work in, and the price is that the two sides are two places: what
// Composer wrote inside has to be brought out, and what an editor wrote outside has to be taken in.
type Transfer struct {
	Engine ports.ContainerEngine
	Files  ports.FileTransfer

	// Runner runs as the container's own user, which is who PHP is: a directory it makes for a
	// copy to land in belongs to PHP from the start.
	Runner ports.ContainerRunner

	// Root is how the ownership is put right afterwards: a file copied in belongs to root, PHP
	// runs as somebody else, and only root may give a file away.
	Root ports.ContainerRunner

	// Workdir is where the code lives inside the container.
	Workdir string

	Announce func(string)
	Binary   string

	// Watch shows each path's copy while it happens. Nil, which is what anything but a person at
	// a terminal gets, copies exactly as before: nothing measured, nothing asked inside the
	// container.
	Watch ports.CopyWatch
}

// Into copies paths from the project into the container.
//
// A path that is a bind mount inside the container is refused rather than copied: the two sides
// are already the same file, and copying one onto the other is a way to lose whichever was newer.
func (t Transfer) Into(project core.Project, paths []string, owner string) error {
	container, err := t.container(project)
	if err != nil {
		return err
	}

	for _, one := range paths {
		source := filepath.Join(project.Root, project.MagentoDir, one)

		// Something that is not there is skipped rather than refused: these are lists of paths a
		// project may or may not have, and the caller is asking for whichever it does
		if _, err := os.Stat(source); err != nil {
			continue
		}

		if mount := t.boundAt(container, path.Join(t.workdir(), one)); mount != "" {
			return core.Refusal{
				Kind:    "path_is_bound",
				Code:    6,
				Message: fmt.Sprintf("'%s' cannot be copied: it is a bind mount inside the container", one),
				Hint:    "The two sides are already the same file, at " + mount,
			}
		}

		// Into the directory that holds it, which is what the daemon's copy expects and what
		// makes a file land beside its neighbours rather than inside itself
		target := path.Dir(path.Join(t.workdir(), one))

		// "." is the whole project: its contents go into the workdir itself, as `docker cp ./.`
		// sends them, and not the project directory into the workdir's parent
		if path.Clean(filepath.ToSlash(one)) == "." {
			source += string(filepath.Separator) + "."
			target = t.workdir()
		}

		// Made first, because the daemon's copy needs somewhere to put it and answers "could not
		// find the file" about the destination when there is none — which reads as though what
		// was being copied did not exist
		if err := t.mkdir(container, target); err != nil {
			return err
		}

		t.say(fmt.Sprintf("Copying %s -> phpfpm:%s\n", one, target))

		if err := t.into(container, source, target); err != nil {
			return err
		}

		if err := t.own(container, path.Join(t.workdir(), one), owner); err != nil {
			return err
		}
	}

	return nil
}

// All copies the whole project in, which is what an environment that has just been created needs.
func (t Transfer) All(project core.Project, owner string) error {
	container, err := t.container(project)
	if err != nil {
		return err
	}

	t.say("Copying everything -> phpfpm:" + t.workdir() + "\n")

	source := filepath.Join(project.Root, project.MagentoDir)

	if err := t.into(container, source+string(filepath.Separator)+".", t.workdir()); err != nil {
		return err
	}

	return t.own(container, t.workdir(), owner)
}

// From copies paths out of the container into the project.
func (t Transfer) From(project core.Project, paths []string) error {
	// Stopped is enough: copying out is a read of the container's filesystem, which the daemon
	// serves either way, and Composer's macOS dance stops phpfpm on purpose before this copy so
	// that nothing writes to the tree while it is read
	found, err := anyContainer(t.Engine, project, phpService, t.Binary)
	if err != nil {
		return err
	}

	container, running := found.ID, found.Running

	// What this call has already copied, by the name at the top of the workdir: the whole workdir
	// asked after vendor — Composer's dance, vendor first so that a failure later in the tree does
	// not leave the host's emptied vendor empty — leaves vendor out rather than writing it twice
	var copied []string

	for _, one := range paths {
		one = strings.TrimSuffix(one, "/")

		var skip []string
		if path.Clean(one) == "." {
			skip = copied
		} else {
			copied = append(copied, strings.SplitN(path.Clean(one), "/", 2)[0])
		}

		source := path.Join(t.workdir(), one)
		target := filepath.Join(project.Root, project.MagentoDir, one)

		// A directory is copied as its contents, so that copying `generated` onto an existing
		// `generated` replaces what is in it rather than nesting one inside the other
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			source += "/."
		}

		t.say(fmt.Sprintf("Copying phpfpm:%s -> %s\n", one, one))

		if err := t.from(container, source, target, running, skip); err != nil {
			return err
		}
	}

	return nil
}

// skippingFileTransfer is a copy out that can leave entries at the top of the copied tree
// unwritten. Optional, like the metered copy: a transfer without it copies the tree whole, which
// is slower and still right.
type skippingFileTransfer interface {
	FromSkipping(container, source, target string, skip []string, meter ports.CopyMeter) error
}

// into copies one path in, shown while it happens when somebody is watching, against the total
// measured here: what is on this machine is what is about to be sent.
func (t Transfer) into(container, source, target string) error {
	metered, ok := t.Files.(ports.MeteredFileTransfer)
	if t.Watch == nil || !ok {
		return t.Files.Into(container, source, target)
	}

	meter, end := t.Watch(hostSize(source))
	err := metered.IntoMetered(container, source, target, meter)
	end(err != nil)

	return err
}

// from copies one path out, shown while it happens when somebody is watching. The total is asked
// of the container only while it runs; a stopped one cannot run du, and is shown as a counter.
func (t Transfer) from(container, source, target string, running bool, skip []string) error {
	carry := func(meter ports.CopyMeter) error {
		return t.Files.(ports.MeteredFileTransfer).FromMetered(container, source, target, meter)
	}

	skipping, skips := t.Files.(skippingFileTransfer)
	_, metered := t.Files.(ports.MeteredFileTransfer)

	switch {
	case skips && len(skip) > 0:
		carry = func(meter ports.CopyMeter) error {
			return skipping.FromSkipping(container, source, target, skip, meter)
		}
	case t.Watch == nil || !metered:
		return t.Files.From(container, source, target)
	}

	if t.Watch == nil {
		return carry(nil)
	}

	var total int64
	if running {
		total = t.containerSize(container, strings.TrimSuffix(source, "/."))
	}

	meter, end := t.Watch(total)
	err := carry(meter)
	end(err != nil)

	return err
}

// containerSize is what du says a path inside the container weighs, or 0 when it says anything
// else: the total only decides whether a percentage is shown, never whether the copy is made.
func (t Transfer) containerSize(container, target string) int64 {
	var answer strings.Builder

	code, err := t.Runner.Run(container, []string{"du", "-sb", target}, nil, &answer)
	if err != nil || code != 0 {
		return 0
	}

	fields := strings.Fields(answer.String())
	if len(fields) == 0 {
		return 0
	}

	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || size < 0 {
		return 0
	}

	return size
}

// hostSize is the bytes a copy of source will carry, counted the way the copy reads them: a link
// is followed to what it points at, a link to nothing is skipped, and a linked directory is not
// walked into. Anything unreadable is left out rather than failing — it is an estimate.
func hostSize(source string) int64 {
	var total int64

	filepath.Walk(filepath.Clean(source), func(path string, entry os.FileInfo, err error) error { //nolint:errcheck
		if err != nil {
			return nil //nolint:nilerr
		}

		if entry.Mode()&os.ModeSymlink != 0 {
			if entry, err = os.Stat(path); err != nil {
				return nil //nolint:nilerr
			}
		}

		if entry.Mode().IsRegular() {
			total += entry.Size()
		}

		return nil
	})

	return total
}

// IntoService copies one file from this machine into a directory of another service's container: the
// dump transfer-db brought over SSH, into the database container that restores it. Watched like
// any copy in: a dump of several gigabytes is as long a silence as a vendor directory.
func (t Transfer) IntoService(project core.Project, service, source, target string) error {
	container, err := resolveContainer(t.Engine, project, service, t.Binary)
	if err != nil {
		return err
	}

	return t.into(container, source, target)
}

// mkdir makes a directory inside the container, and says nothing when it is already there.
func (t Transfer) mkdir(container, target string) error {
	code, err := t.Runner.Run(container, []string{"sh", "-c", "mkdir -p " + target}, nil, io.Discard)
	if err != nil {
		return err
	}

	if code != 0 {
		return fmt.Errorf("mkdir -p %s inside the container exited with %d", target, code)
	}

	return nil
}

// own puts the ownership right, because a file the daemon wrote belongs to root and PHP does not
// run as root.
func (t Transfer) own(container, target, owner string) error {
	if owner == "" {
		return nil
	}

	code, err := t.Root.Run(container,
		[]string{"sh", "-c", "chown -R " + owner + " " + target}, nil, os.Stderr)
	if err != nil {
		return err
	}

	// Ignored, this is a copy that looks finished and a Composer that later cannot write a file
	if code != 0 {
		return fmt.Errorf("chown -R %s %s inside the container exited with %d", owner, target, code)
	}

	return nil
}

// boundAt reports where a container path comes from the host, or nothing when it does not.
func (t Transfer) boundAt(container, target string) string {
	containers, err := t.Engine.Containers()
	if err != nil {
		return ""
	}

	for _, candidate := range containers {
		if candidate.ID != container {
			continue
		}

		for _, mount := range candidate.Mounts {
			if mount.Type != "bind" {
				continue
			}

			if overlaps(mount.Destination, target) || overlaps(target, mount.Destination) {
				return mount.Destination
			}
		}
	}

	return ""
}

// container is this project's php container, refusing when it is not running: copying in ends
// with a chown run inside it.
func (t Transfer) container(project core.Project) (string, error) {
	return resolveContainer(t.Engine, project, phpService, t.Binary)
}

func (t Transfer) workdir() string {
	if t.Workdir == "" {
		return "/var/www/html"
	}

	return t.Workdir
}

func (t Transfer) say(message string) {
	if t.Announce != nil {
		t.Announce(message)
	}
}

// phpService is the service the code runs in, named here as it is named everywhere else.
const phpService = "phpfpm"
