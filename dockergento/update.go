package dockergento

import (
	"io/fs"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Update replaces the running executable with the newest release, when it is newer than the running
// version: final releases only, unless pre-releases are asked for. A running version newer than
// every release — a release candidate ahead of the last final release — is up to date: an update
// never moves back.
//
// A binary built from a checkout has no release version to compare, and is refused: it is updated
// where it was built.
func (e *Engine) Update(options core.UpdateOptions) (core.Updated, error) {
	running, err := e.runningRelease(options.Running)
	if err != nil {
		return core.Updated{}, err
	}

	install := e.installer().attended(options.Unattended)
	looking := e.step("Looking up the latest release")

	found, err := e.releases(install.source, options.PreReleases)
	if err != nil {
		looking("", true)

		return core.Updated{}, err
	}

	if len(found) == 0 || !running.less(found[0].version) {
		// The answer is the command's own, up to date: the step leaves nothing to repeat it
		looking("", false)

		latest := ""
		if len(found) > 0 {
			latest = found[0].Tag
		}

		path, _ := install.target()

		return core.Updated{From: options.Running, To: options.Running, Latest: latest, Path: path}, nil
	}

	looking(lookedUp(found[0], options.Running, false), false)

	path, err := e.put(install, found[0])
	if err != nil {
		return core.Updated{}, err
	}

	return core.Updated{Updated: true, From: options.Running, To: found[0].Tag, Latest: found[0].Tag, Path: path}, nil
}

// AgentPermissions is the configuration an agent using this tool should be given, from the
// tool's own command_descriptions.json and ai-exclusions.json.
func (e *Engine) AgentPermissions(strict bool) (core.AgentPermissions, error) {
	commands, err := fs.ReadFile(e.options.Assets, "data/command_descriptions.json")
	if err != nil {
		return core.AgentPermissions{}, err
	}

	exclusions, err := fs.ReadFile(e.options.Assets, "data/ai-exclusions.json")
	if err != nil {
		return core.AgentPermissions{}, err
	}

	return core.ReadAgentPermissions(commands, exclusions, e.options.Binary, strict)
}
