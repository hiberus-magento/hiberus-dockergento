package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// AgentContext writes what an agent needs to know about a project where it will read it.
//
// Everything in it is already answerable — the description has resolved the versions, the URLs
// and the admin's real front name. What was missing is that anybody thought to ask: an agent that
// is not told the command exists works the facts out from docker-compose.yml and composer.lock,
// and gets the configured value instead of the resolved one, or invents /admin and reports the 404
// as a bug. The same file says what not to read: app/etc/env.php holds the encryption key, var/log
// whatever the site logged about real customers, and an agent reads a project by default.
//
// Ported from console/commands/ai-context.sh, and the files it writes are that script's byte for
// byte, with one difference on purpose: the MCP server it wires up is the binary, not bin/run.
type AgentContext struct {
	Daemon ports.Daemon

	// Describe is the project, resolved. It is asked once, after the daemon has answered.
	Describe func() (core.Description, error)

	// Assets is the files the tool ships with, whose data/ai-exclusions.json says what an agent
	// is not to read.
	Assets fs.FS

	// Command is what .mcp.json starts the MCP server with: the running executable. Empty, when it
	// could not be worked out, means Binary, found on PATH.
	Command string

	// Binary is the name the tool was invoked as, and Version what the block says generated it.
	Binary  string
	Version string
}

// The line a CLAUDE.md needs to load the context. Matched as grep matched it, a regular
// expression, which is why the dot is not escaped: a file grep accepted is accepted here too.
var loadsAgentContext = regexp.MustCompile(`AGENTS.md`)

// Write writes AGENTS.md, CLAUDE.md when there is none, and .mcp.json, into the project's root.
func (a AgentContext) Write() (core.AgentContextResult, error) {
	if !a.Daemon.Reachable() {
		return core.AgentContextResult{}, core.Refusal{
			Kind: "docker_unavailable", Code: contract.ExitDocker,
			Message: "Docker is not running", Hint: "Start Docker and try again",
		}
	}

	description, err := a.Describe()
	if err != nil {
		// Passed on as it is, as describe does: a refusal is already the project's own answer
		// (configuration unreadable, no name to derive), and any other error is the engine's or
		// the daemon's, which project_not_configured would misname
		return core.AgentContextResult{}, err
	}

	if description.Project.Name == "" {
		return core.AgentContextResult{}, core.Refusal{
			Kind: "no_project", Code: contract.ExitProject,
			Message: "There is no project here to describe", Hint: a.Binary + " setup",
		}
	}

	shipped, err := fs.ReadFile(a.Assets, "data/ai-exclusions.json")
	if err != nil {
		return core.AgentContextResult{}, err
	}

	exclusions, err := core.ReadAgentExclusions(shipped)
	if err != nil {
		return core.AgentContextResult{}, err
	}

	root := description.Project.Root
	block := core.AgentContext{
		Binary: a.Binary, Version: a.Version, Description: description, Exclusions: exclusions,
	}.Block()

	if err := writeAgentsFile(filepath.Join(root, "AGENTS.md"), block); err != nil {
		if errors.Is(err, core.ErrAgentContextNotClosed) {
			return core.AgentContextResult{}, core.Refusal{
				Kind: "block_not_closed", Code: contract.ExitBlocked,
				Message: "AGENTS.md has an opening marker with no closing one, so the block cannot be replaced",
				Hint:    "Restore the " + core.AgentContextEnd + " line, or delete the block and run this again",
			}
		}

		return core.AgentContextResult{}, err
	}

	result := core.AgentContextResult{Written: []string{"AGENTS.md"}, Root: root}

	//
	// A CLAUDE.md is the first thing somebody wrote about their project. It is created when it is
	// missing and never touched when it is not: the line to add is reported instead.
	//
	claude := filepath.Join(root, "CLAUDE.md")

	if !isRegularFile(claude) {
		if err := os.WriteFile(claude, []byte("@AGENTS.md\n"), 0o666); err != nil { //nolint:gosec // as a shell redirection creates it
			return core.AgentContextResult{}, err
		}

		result.Written = append(result.Written, "CLAUDE.md")
	} else if contents, err := os.ReadFile(claude); err == nil && !loadsAgentContext.Match(contents) {
		needs := "@AGENTS.md"
		result.ClaudeNeeds = &needs
	}

	// An executable that could not be worked out is never an empty command, which no agent can
	// start: the binary's name is, found on PATH as the user's shell finds it
	command := a.Command
	if command == "" {
		command = a.binaryName()
	}

	if err := writeMCPFile(filepath.Join(root, ".mcp.json"), a.binaryName(), command, root); err != nil {
		return core.AgentContextResult{}, err
	}

	result.Written = append(result.Written, ".mcp.json")

	return result, nil
}

func (a AgentContext) binaryName() string {
	if a.Binary == "" {
		return "hm"
	}

	return a.Binary
}

// writeAgentsFile puts the block into AGENTS.md.
//
// A replacement goes through a temporary file beside it and a rename: never the file truncated
// and rewritten in place, where a failure halfway leaves somebody's notes half gone. The temporary
// file is created owner-only, as mktemp created it, and the renamed file keeps that.
func writeAgentsFile(path, block string) error {
	var existing []byte

	present := isRegularFile(path)
	if present {
		read, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		existing = read
	}

	placement, contents, err := core.PlaceAgentContext(existing, present, block)
	if err != nil {
		return err
	}

	switch placement {
	case core.PlacementCreate:
		return os.WriteFile(path, contents, 0o666) //nolint:gosec // as a shell redirection creates it
	case core.PlacementAppend:
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return err
		}

		if _, err := file.Write(contents); err != nil {
			file.Close() //nolint:errcheck,gosec // the write error is the one worth reporting

			return err
		}

		return file.Close()
	default:
		return replaceFile(path, contents)
	}
}

// writeMCPFile merges this tool's server into .mcp.json, always through a temporary file.
func writeMCPFile(path, name, command, directory string) error {
	// One that cannot be read is one jq could not parse, and that was treated as empty
	existing, err := os.ReadFile(path)
	present := err == nil && isRegularFile(path)

	contents, write := core.MergeAgentMCP(existing, present, name, command, directory)
	if !write {
		return nil
	}

	return replaceFile(path, contents)
}

func replaceFile(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".")
	if err != nil {
		return err
	}

	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()           //nolint:errcheck,gosec // the write error is the one worth reporting
		os.Remove(temporary.Name()) //nolint:errcheck,gosec // best effort, and the error above says why

		return err
	}

	if err := temporary.Close(); err != nil {
		os.Remove(temporary.Name()) //nolint:errcheck,gosec // best effort, and the error above says why

		return err
	}

	return os.Rename(temporary.Name(), path)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}
