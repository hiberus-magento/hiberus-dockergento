package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// `hm ai-context` writes three files, and the shell implementation it replaces wrote them first:
// the AGENTS.md sitting in people's checkouts came from it. These tests hold the port to what it
// wrote, byte for byte, and to the one reader that depends on it — the doctor's staleness check.
//

// agentScene is one project as both implementations see it: the facts behind the description and
// the files already in the project's directory.
type agentScene struct {
	name string

	magento    magento
	services   []core.Service
	containers []core.Container
	domain     string
	anonymised string // when the data was anonymised, "" for never
	binary     string
	version    string // HM_VERSION, "" when it is not set

	// files are what the directory holds before the command runs, with their modes.
	files map[string]agentFile
}

type agentFile struct {
	contents string
	mode     fs.FileMode
}

// writer is the port, over the same fakes app.Describer is tested with: the description it
// writes from is a real one, resolved from those facts.
func (s agentScene) writer(t *testing.T, root, repository string) AgentContext {
	t.Helper()

	project := core.Project{Name: "shop", Root: root, Domain: s.domain, MagentoDir: "."}

	describer := Describer{
		Engine:  &engine{containers: s.containers},
		Compose: compose{configuration: core.Compose{Services: s.services}},
		Magento: s.magento,
		Tooling: tools{},
		State:   state{at: s.anonymised},
	}

	return AgentContext{
		Daemon: daemon{reachable: true},
		Describe: func() (core.Description, error) {
			return describer.Describe(project, core.ComposeFiles{}, false)
		},
		Assets:  os.DirFS(repository),
		Command: filepath.Join(repository, "bin", "hm"),
		Binary:  s.binary,
		Version: s.version,
	}
}

// doctor is the staleness check over the same facts, reading the AGENTS.md on disk.
func (s agentScene) doctor(root string) Doctor {
	physician := doctorFor(true)
	physician.Project = core.Project{Name: "shop", Root: root, Domain: s.domain, MagentoDir: "."}
	physician.Magento = s.magento
	physician.Compose = compose{configuration: core.Compose{Services: s.services}}
	physician.State = state{at: s.anonymised}
	physician.FS = disk{}

	return physician
}

// disk is the doctor's FS over real files, which is where the context it reads was written.
type disk struct{}

func (disk) IsDir(path string) bool  { info, err := os.Stat(path); return err == nil && info.IsDir() }
func (disk) Exists(path string) bool { _, err := os.Stat(path); return err == nil }
func (disk) MkdirAll(string) error   { return nil }

func (disk) Read(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(contents)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func seed(t *testing.T, root string, files map[string]agentFile) {
	t.Helper()

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, file := range files {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(file.contents), file.mode); err != nil {
			t.Fatal(err)
		}

		// WriteFile's mode goes through the umask; the scene's is the one meant
		if err := os.Chmod(path, file.mode); err != nil {
			t.Fatal(err)
		}
	}
}

// snapshot is every file in the directory with its mode: what was written, and nothing left
// beside it.
func snapshot(t *testing.T, root string) map[string]agentFile {
	t.Helper()

	found := map[string]agentFile{}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}

		contents, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}

		found[entry.Name()] = agentFile{contents: string(contents), mode: info.Mode().Perm()}
	}

	return found
}

func agentScenes() []agentScene {
	plain := agentScene{
		name:    "a plain project, nothing written yet",
		magento: magento{version: "2.4.7-p3", mode: "developer", admin: "admin_9k2"},
		services: []core.Service{
			{Name: "db", Image: "hiberusmagento/mariadb:10.6"},
			{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"},
		},
		domain: "shop.test",
		binary: "hm",
	}

	// Every branch the block has taken the other way, and files of somebody's in all three places
	changed := agentScene{
		name:    "anonymised, no domain, no version, files already there",
		magento: magento{},
		services: []core.Service{
			// In name order, as the Compose loader hands them over
			{Name: "db", Image: "mariadb:10.6 & <friends>"},
			{Name: "mailpit", Image: "hiberusmagento/mailpit:1.21", Ports: []core.Port{{Published: "8025", Target: "8025"}}},
			{Name: "phpfpm", Image: "hiberusmagento/php:8.2-fpm"},
		},
		containers: []core.Container{
			{ComposeProject: "shop", ComposeService: "phpfpm", Running: true, StateName: "running"},
			{ComposeProject: "shop", ComposeService: "db", Running: false, StateName: "exited"},
		},
		anonymised: "2026-11-02 10:15",
		binary:     "hm2",
		version:    "1.6.0-rc.2",
		files: map[string]agentFile{
			"AGENTS.md": {mode: 0o644, contents: "Before the block.\n\n" + core.AgentContextBegin +
				"\nan old block\n" + core.AgentContextEnd + "\n\n## Mine\n\nAfter it, with no newline at the end"},
			"CLAUDE.md": {mode: 0o640, contents: "My own instructions.\n"},
			".mcp.json": {mode: 0o644, contents: `{"first": 1.0, "mcpServers": {"other": {"command": "x", "env": {"A": "é<>& \u0001\u007f/"}}, "hm2": {"command": "old"}, "last": [true, false, null, [], {}]}, "z": 100000000000000000001}`},
		},
	}

	appended := agentScene{
		name:     "an AGENTS.md with no block and no final newline, an unreadable .mcp.json",
		magento:  magento{version: "2.4.6", mode: "production", admin: "admin"},
		services: []core.Service{{Name: "phpfpm", Image: "hiberusmagento/php:8.1-fpm"}},
		domain:   "appended.test",
		binary:   "hm",
		files: map[string]agentFile{
			"AGENTS.md": {mode: 0o640, contents: "# Notes\n\nNo block here"},
			// grep's dot matches anything, so this one loads the context as far as grep can tell
			"CLAUDE.md": {mode: 0o644, contents: "see AGENTSxmd\n"},
			".mcp.json": {mode: 0o644, contents: `{"mcpServers": {`},
		},
	}

	empty := agentScene{
		name:     "an empty .mcp.json and a block at the very top",
		magento:  magento{version: "2.4.7", mode: "developer", admin: "admin"},
		services: []core.Service{{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"}},
		domain:   "empty.test",
		binary:   "hm",
		files: map[string]agentFile{
			"AGENTS.md": {mode: 0o600, contents: core.AgentContextBegin + "\n" + core.AgentContextEnd + "\n"},
			".mcp.json": {mode: 0o644, contents: "  \n"},
		},
	}

	several := agentScene{
		name:     "several documents in .mcp.json, one of them null",
		magento:  magento{version: "2.4.7", mode: "developer", admin: "admin"},
		services: []core.Service{{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"}},
		domain:   "several.test",
		binary:   "hm",
		files: map[string]agentFile{
			".mcp.json": {mode: 0o644, contents: "{\"a\": 1}\nnull\n{\"mcpServers\": null, \"a\": {\"a\": 1, \"a\": 2}}"},
		},
	}

	unclosed := agentScene{
		name:     "a block that was opened and never closed",
		magento:  magento{version: "2.4.7", mode: "developer", admin: "admin"},
		services: []core.Service{{Name: "phpfpm", Image: "hiberusmagento/php:8.3-fpm"}},
		domain:   "unclosed.test",
		binary:   "hm",
		files: map[string]agentFile{
			"AGENTS.md": {mode: 0o644, contents: "Mine.\n" + core.AgentContextBegin + "\nhalf a block\n"},
		},
	}

	reversed := unclosed
	reversed.name = "a closing marker above the opening one"
	reversed.files = map[string]agentFile{
		"AGENTS.md": {mode: 0o644, contents: core.AgentContextEnd + "\nMine.\n" + core.AgentContextBegin + "\n"},
	}

	return []agentScene{plain, changed, appended, empty, several, unclosed, reversed}
}

// TestTheDoctorAcceptsTheContextItWrites: the block's fingerprint is the one the staleness check
// recomputes, and it stops matching when a fact behind it changes.
func TestTheDoctorAcceptsTheContextItWrites(t *testing.T) {
	repository := repositoryRoot(t)

	for _, scene := range agentScenes()[:3] {
		t.Run(scene.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "shop")
			seed(t, root, scene.files)

			if _, err := scene.writer(t, root, repository).Write(); err != nil {
				t.Fatalf("Write: %v", err)
			}

			found := findingsFor(scene.doctor(root).Diagnose("agent-context"), "agent-context")
			if len(found) != 1 || found[0].Severity != core.SeverityOK ||
				found[0].Message != "The agent context matches this project" {
				t.Fatalf("the doctor said %+v about the context just written, want it current", found)
			}

			moved := scene
			moved.magento.admin = "somewhere-else"
			moved.domain = "moved.test"

			found = findingsFor(moved.doctor(root).Diagnose("agent-context"), "agent-context")
			if len(found) != 1 || found[0].Severity != core.SeverityError {
				t.Errorf("the doctor said %+v after the admin moved, want the context stale", found)
			}
		})
	}
}

// TestAnMCPFileThatIsNotAnObjectIsLeftAlone is the one edge where the port does not copy the
// shell implementation: jq failed on it, left it as it was and a temporary file beside it.
func TestAnMCPFileThatIsNotAnObjectIsLeftAlone(t *testing.T) {
	scene := agentScenes()[0]
	scene.files = map[string]agentFile{".mcp.json": {mode: 0o644, contents: "[1, 2]\n"}}

	root := filepath.Join(t.TempDir(), "shop")
	seed(t, root, scene.files)

	result, err := scene.writer(t, root, repositoryRoot(t)).Write()
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	after := snapshot(t, root)

	if after[".mcp.json"] != scene.files[".mcp.json"] {
		t.Errorf(".mcp.json = %+v, want it untouched", after[".mcp.json"])
	}

	if len(after) != 3 {
		t.Errorf("the directory holds %d files, want AGENTS.md, CLAUDE.md and .mcp.json and nothing beside them", len(after))
	}

	if !cmp.Equal(result.Written, []string{"AGENTS.md", "CLAUDE.md", ".mcp.json"}) {
		t.Errorf("written = %v, want all three named, as the shell implementation named them", result.Written)
	}
}

// TestNothingIsWrittenWithoutADaemonOrAProject: the two refusals before anything is touched, with
// the codes bin/run answered them with.
func TestNothingIsWrittenWithoutADaemonOrAProject(t *testing.T) {
	cases := []struct {
		name     string
		reach    bool
		describe error
		kind     string
		code     int
	}{
		{"no daemon", false, nil, "docker_unavailable", contract.ExitDocker},
		{"no configuration", true, NotConfigured("no docker-compose.yml", "hm"), "project_not_configured", contract.ExitProject},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "shop")
			seed(t, root, nil)

			writer := agentScenes()[0].writer(t, root, repositoryRoot(t))
			writer.Daemon = daemon{reachable: c.reach}

			if c.describe != nil {
				writer.Describe = func() (core.Description, error) { return core.Description{}, c.describe }
			}

			_, err := writer.Write()

			var refusal core.Refusal
			if !errors.As(err, &refusal) || refusal.Kind != c.kind || refusal.Code != c.code {
				t.Fatalf("Write = %v, want the %s refusal with code %d", err, c.kind, c.code)
			}

			if len(snapshot(t, root)) != 0 {
				t.Errorf("files were written: %v", snapshot(t, root))
			}
		})
	}
}

// When the running executable cannot be worked out, .mcp.json never gets an empty command, which
// no agent can start: it names the binary, found on PATH.
func TestAnUnknownExecutableIsNeverAnEmptyMCPCommand(t *testing.T) {
	scene := agentScenes()[0]
	scene.binary = "hm"

	root := filepath.Join(t.TempDir(), "shop")
	seed(t, root, scene.files)

	writer := scene.writer(t, root, repositoryRoot(t))
	writer.Command = ""

	if _, err := writer.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(written), `"command": ""`) || !strings.Contains(string(written), `"command": "hm"`) {
		t.Errorf(".mcp.json = %s, want the command hm", written)
	}
}

// A description that is already refused (the configuration unreadable, a directory with no
// name) is passed on as it is: wrapping it again would repeat its sentence, or turn
// no_project_name into project_not_configured.
func TestARefusedDescriptionIsPassedOnAsItIs(t *testing.T) {
	for _, refused := range []core.Refusal{
		{Kind: "project_not_configured", Code: contract.ExitProject,
			Message: "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid: no compose file",
			Hint:    "hm setup"},
		{Kind: "no_project_name", Code: contract.ExitProject,
			Message: "No project name can be derived from /x/___", Hint: "Rename the directory"},
	} {
		t.Run(refused.Kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "shop")
			seed(t, root, nil)

			writer := agentScenes()[0].writer(t, root, repositoryRoot(t))
			writer.Daemon = daemon{reachable: true}
			writer.Describe = func() (core.Description, error) { return core.Description{}, refused }

			_, err := writer.Write()

			var got core.Refusal
			if !errors.As(err, &got) || got != refused {
				t.Fatalf("Write = %#v, want the description's own refusal %#v", err, refused)
			}

			if len(snapshot(t, root)) != 0 {
				t.Errorf("files were written: %v", snapshot(t, root))
			}
		})
	}
}

// An error that is not a refusal is the engine's or the daemon's, and goes out as it is, as
// describe sends it: calling it project_not_configured would send somebody to `hm setup` for a
// daemon that stopped answering.
func TestAnEngineErrorIsNotCalledAnUnconfiguredProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	seed(t, root, nil)

	broken := errors.New("Cannot connect to the Docker daemon")

	writer := agentScenes()[0].writer(t, root, repositoryRoot(t))
	writer.Daemon = daemon{reachable: true}
	writer.Describe = func() (core.Description, error) { return core.Description{}, broken }

	_, err := writer.Write()

	var refusal core.Refusal
	if !errors.Is(err, broken) || errors.As(err, &refusal) {
		t.Fatalf("Write = %#v, want the engine's own error, not a refusal", err)
	}

	if len(snapshot(t, root)) != 0 {
		t.Errorf("files were written: %v", snapshot(t, root))
	}
}
