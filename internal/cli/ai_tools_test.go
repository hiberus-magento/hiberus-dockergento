package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/app"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-init, ai-pull and ai-reset against real directories: the project is the test's working
// directory, the tool beside it is a temporary installation with its own data/ and skills/, and
// a "download" unpacks a fixture repository instead of reaching the network.
//

// aiNow is the time every configuration and registration entry in these tests is recorded at.
var aiNow = time.Date(2026, 11, 20, 8, 30, 0, 0, time.UTC)

// aiFixture is a project, the tool it uses, and what was downloaded.
type aiFixture struct {
	fake         *fakeEngine
	project      string
	tool         string
	downloaded   []string
	failDownload bool
}

func plantFile(t *testing.T, path, contents string, mode fs.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func aiTooling(t *testing.T) *aiFixture {
	t.Helper()

	fixture := &aiFixture{fake: answering(t), project: here(), tool: t.TempDir()}
	t.Setenv("NO_COLOR", "1")

	data := filepath.Join(fixture.tool, "data")
	plantFile(t, filepath.Join(data, "ai-platforms.json"), `{"platforms": {
		"claude": {"name": "Claude Code", "description": "Claude Code (CLI, Desktop, Web)",
		           "skills_dir": ".claude/skills", "agents_dir": ".claude/agents"},
		"cursor": {"name": "Cursor AI Editor", "description": "Cursor AI Editor",
		           "skills_dir": ".cursor/skills", "agents_dir": ".cursor/agents"}
	}}`, 0o644)
	plantFile(t, filepath.Join(data, "ai-skill-types.json"), `{"skill_types": {
		"hyva": {"name": "Hyvä Themes", "description": "Hyvä Themes development"},
		"magento": {"name": "Magento 2", "description": "General Magento 2 development"}
	}}`, 0o644)
	plantFile(t, filepath.Join(data, "ai-repositories.json"), `{"repositories": [
		{"name": "dockergento", "local": true, "types": ["dockergento"]},
		{"name": "remote-tools", "url": "https://github.com/acme/remote-tools", "branch": "main"}
	]}`, 0o644)

	// What comes with the tool
	plantFile(t, filepath.Join(fixture.tool, "skills", "dockergento-environment", "SKILL.md"), "# environment\n", 0o644)
	plantFile(t, filepath.Join(fixture.tool, "skills", "dockergento-database", "SKILL.md"), "# database\n", 0o644)

	// What a remote repository holds
	remote := t.TempDir()
	plantFile(t, filepath.Join(remote, "skills", "hyva-checkout", "SKILL.md"), "# hyva\n", 0o644)
	plantFile(t, filepath.Join(remote, "skills", "magento-orders", "SKILL.md"), "# magento\n", 0o644)
	plantFile(t, filepath.Join(remote, "skills", "magento-orders", "scripts", "run.sh"), "#!/bin/sh\n", 0o755)
	plantFile(t, filepath.Join(remote, "skills", "other-thing", "SKILL.md"), "# other\n", 0o644)
	plantFile(t, filepath.Join(remote, "agents", "reviewer.md"), "# reviewer\n", 0o644)

	fixture.fake.aiTools = &app.AITools{
		Tool:         os.DirFS(fixture.tool),
		Repositories: filepath.Join(data, "ai-repositories.json"),
		Binary:       "hm",
		Now:          func() time.Time { return aiNow },
		Version:      func() string { return "2.0.0-test" },
		Sleep:        func(time.Duration) {},
		HasGit:       func() bool { return true },
		TempRoot:     t.TempDir(),
		Exec: func(_ bool, name string, args ...string) error {
			switch name {
			case "curl":
				fixture.downloaded = append(fixture.downloaded, args[len(args)-1])
				if fixture.failDownload {
					return os.ErrNotExist
				}

				return os.WriteFile(args[len(args)-2], []byte("archive"), 0o644)
			case "tar":
				return copyFixture(remote, args[3])
			default:
				return os.ErrNotExist
			}
		},
	}

	return fixture
}

func copyFixture(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, _ := filepath.Rel(source, path)
		destination := filepath.Join(target, relative)

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destination, contents, info.Mode().Perm())
	})
}

func (f *aiFixture) path(name string) string { return filepath.Join(f.project, name) }

func (f *aiFixture) read(t *testing.T, name string) string {
	t.Helper()

	contents, err := os.ReadFile(f.path(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return string(contents)
}

func (f *aiFixture) exists(name string) bool {
	_, err := os.Stat(f.path(name))

	return err == nil
}

func (f *aiFixture) mode(t *testing.T, name string) fs.FileMode {
	t.Helper()

	info, err := os.Stat(f.path(name))
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}

	return info.Mode().Perm()
}

func (f *aiFixture) digest(name string) string {
	return core.DigestPath(os.DirFS(f.project), name)
}

// run runs a command with what standard input says, and returns its streams and exit code.
func (f *aiFixture) run(t *testing.T, input string, args ...string) (string, string, int) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		writer.WriteString(input) //nolint:errcheck
		writer.Close()            //nolint:errcheck
	}()

	original := os.Stdin
	os.Stdin = reader

	t.Cleanup(func() { os.Stdin = original })

	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)

	os.Stdin = original

	return stdout.String(), stderr.String(), code
}

// configure writes the configuration a person has after ai-init.
func (f *aiFixture) configure(t *testing.T, contents string) {
	t.Helper()
	plantFile(t, f.path(core.AIPropertiesFile), contents, 0o644)
}

func lines(text ...string) string { return strings.Join(text, "\n") + "\n" }
