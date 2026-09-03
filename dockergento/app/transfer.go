package app

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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

	// Runner is how the ownership is put right afterwards: a file copied in belongs to root, and
	// PHP runs as somebody else.
	Runner ports.ContainerRunner

	// Workdir is where the code lives inside the container.
	Workdir string

	Announce func(string)
	Binary   string
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

		// Made first, because the daemon's copy needs somewhere to put it and answers "could not
		// find the file" about the destination when there is none — which reads as though what
		// was being copied did not exist
		if err := t.mkdir(container, target); err != nil {
			return err
		}

		t.say(fmt.Sprintf("Copying %s -> phpfpm:%s\n", one, target))

		if err := t.Files.Into(container, source, target); err != nil {
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

	if err := t.Files.Into(container, source+string(filepath.Separator)+".", t.workdir()); err != nil {
		return err
	}

	return t.own(container, t.workdir(), owner)
}

// From copies paths out of the container into the project.
func (t Transfer) From(project core.Project, paths []string) error {
	container, err := t.container(project)
	if err != nil {
		return err
	}

	for _, one := range paths {
		one = strings.TrimSuffix(one, "/")

		source := path.Join(t.workdir(), one)
		target := filepath.Join(project.Root, project.MagentoDir, one)

		// A directory is copied as its contents, so that copying `generated` onto an existing
		// `generated` replaces what is in it rather than nesting one inside the other
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			source += "/."
		}

		t.say(fmt.Sprintf("Copying phpfpm:%s -> %s\n", one, one))

		if err := t.Files.From(container, source, target); err != nil {
			return err
		}
	}

	return nil
}

// mkdir makes a directory inside the container, and says nothing when it is already there.
func (t Transfer) mkdir(container, target string) error {
	_, err := t.Runner.Run(container, []string{"sh", "-c", "mkdir -p " + target}, nil, io.Discard)

	return err
}

// own puts the ownership right, because a file the daemon wrote belongs to root and PHP does not
// run as root.
func (t Transfer) own(container, target, owner string) error {
	if owner == "" {
		return nil
	}

	_, err := t.Runner.Run(container,
		[]string{"sh", "-c", "chown -R " + owner + " " + target}, nil, os.Stderr)

	return err
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

// container is this project's php container, refusing when it is not running.
func (t Transfer) container(project core.Project) (string, error) {
	containers, err := t.Engine.Containers()
	if err != nil {
		return "", err
	}

	for _, candidate := range containers {
		if candidate.Key() != project.Name || candidate.ComposeService != phpService {
			continue
		}

		if !candidate.Running {
			break
		}

		return candidate.ID, nil
	}

	return "", core.Refusal{
		Kind:    "service_not_running",
		Code:    5,
		Message: "The php container of this project is not running",
		Hint:    t.Binary + " start",
	}
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
