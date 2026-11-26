package dockergento

import (
	"fmt"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// Moving the tool between its releases: `hm switch` installs one release in place of the running
// executable, whichever it is, older or newer. Only releases of 2.0 and later — 1.x is the shell
// tool, which has no binary — and only releases: a branch has no binary either, and is built from
// source.
//

// Versions is what the tool can be switched to: every release of 2.0 or later, pre-releases
// included and labelled, newest first, with the running one marked.
func (e *Engine) Versions(running string) (core.Versions, error) {
	found, err := e.releases(e.installer().source, true)
	if err != nil {
		return core.Versions{}, err
	}

	versions := make([]core.Version, 0, len(found))
	for _, item := range found {
		versions = append(versions, core.Version{
			Name:       item.Tag,
			PreRelease: item.PreRelease,
			Installed:  item.Tag == running,
		})
	}

	return core.Versions{Current: orDevel(running), Versions: versions}, nil
}

// Switch installs the release asked for, or the newest final release, in place of the running
// executable. What is asked is checked before anything is read from GitHub: a 1.x or a branch is
// refused without a connection.
func (e *Engine) Switch(options core.SwitchOptions) (core.Switched, error) {
	asked := strings.TrimPrefix(options.Version, "v")

	if !options.Stable {
		version, ok := parseVersion(asked)

		switch {
		case !ok:
			return core.Switched{}, core.Refusal{
				Kind: "not_a_version",
				Code: contract.ExitUsage,
				Message: fmt.Sprintf("'%s' is not a version: %s switch installs releases, and branches can no longer be switched to",
					options.Version, e.options.Binary),
				Hint: "To run a branch, build it from source: git clone -b " + options.Version + " " + repository +
					" && cd hiberus-dockergento && go build -o hm ./cmd/hm",
			}
		case version.major < 2:
			return core.Switched{}, core.Refusal{
				Kind: "version_before_go",
				Code: contract.ExitUsage,
				Message: fmt.Sprintf("'%s' is a version of the shell tool (1.x), which %s switch cannot install: "+
					"nothing was changed", asked, e.options.Binary),
				Hint: "Install it separately, in a directory of its own: git clone -b " + asked +
					" " + repository + " ~/hm-1.x, then run ~/hm-1.x/bin/run",
			}
		}
	}

	if _, err := e.runningRelease(options.Running); err != nil {
		return core.Switched{}, err
	}

	install := e.installer().attended(options.Unattended)

	label := "Looking up release " + asked
	if options.Stable {
		label = "Looking up the latest final release"
	}

	looking := e.step(label)

	found, err := e.releases(install.source, !options.Stable)
	if err != nil {
		looking("", true)

		return core.Switched{}, err
	}

	var target *published

	for i := range found {
		if options.Stable || found[i].Tag == asked {
			target = &found[i]
			break
		}
	}

	if target == nil {
		looking("", true)

		if options.Stable {
			return core.Switched{}, core.Refusal{
				Kind:    "no_release",
				Code:    contract.ExitUsage,
				Message: "No final release of 2.0 or later is published yet: nothing was changed",
				Hint:    e.options.Binary + " switch --list",
			}
		}

		return core.Switched{}, core.Refusal{
			Kind:    "unknown_version",
			Code:    contract.ExitUsage,
			Message: fmt.Sprintf("There is no release called '%s'", options.Version),
			Hint:    e.options.Binary + " switch --list",
		}
	}

	if target.Tag == options.Running {
		// "Already running" is the command's own answer: the step leaves nothing to repeat it
		looking("", false)

		path, _ := install.target()

		return core.Switched{Reference: target.Tag, Version: target.Tag, From: options.Running, Path: path, Unchanged: true}, nil
	}

	looking(lookedUp(*target, options.Running, !options.Stable), false)

	path, err := e.put(install, *target)
	if err != nil {
		return core.Switched{}, err
	}

	return core.Switched{Reference: target.Tag, Version: target.Tag, From: options.Running, Path: path}, nil
}

// ReleaseNotes is where a person reads what version changed.
func ReleaseNotes(version string) string { return releasePage + version }
