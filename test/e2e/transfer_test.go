package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/test/e2e"
)

//
// Moving a project's files between this machine and its container.
//
// It exists because of macOS: there the code is copied into a volume rather than mounted, which is
// what makes PHP fast enough to work in, and the price is that the two sides are two places.
//

// mounted is a project whose container has one directory bound from the host and the rest not,
// which is the shape these commands exist for: what is mounted is already the same file on both
// sides, and what is not has to be carried.
func mounted(t *testing.T) (*e2e.Session, e2e.Project) {
	t.Helper()

	e2e.NeedsDocker(t)

	session := e2e.New(t)
	project := e2e.NewProject(t, "hm-e2e-copias", e2e.Definition{})

	if err := os.MkdirAll(filepath.Join(project.Root, "montado"), 0o755); err != nil {
		t.Fatal(err)
	}

	compose := `services:
  phpfpm:
    image: alpine:latest
    working_dir: /var/www/html
    command: ["sleep", "600"]
    volumes:
      - ` + project.Root + `/montado:/var/www/html/montado
`

	for _, file := range []string{
		"docker-compose.yml",
		"docker-compose.dev.mac.yml",
		"docker-compose.dev.linux.yml",
	} {
		project.Write(t, file, compose)
	}

	e2e.Up(t, session, project)

	return session, project
}

func TestCopyIntoTheContainer(t *testing.T) {
	t.Parallel()

	session, project := mounted(t)

	project.Write(t, "app/code/uno.php", "uno\n")
	project.Write(t, "app/code/vendor-suyo/dos.php", "dos\n")
	project.Write(t, "suelto.txt", "tres\n")

	got := session.Run(t, project.Root, "copy-to-container", "app/code", "suelto.txt")
	if got.Code != 0 {
		t.Fatalf("copy-to-container app/code suelto.txt = %d, want 0\n%s", got.Code, got.Output())
	}

	cases := map[string]struct{ path, want string }{
		"the directory":     {path: "app/code/uno.php", want: "uno"},
		"what was under it": {path: "app/code/vendor-suyo/dos.php", want: "dos"},

		// A file lands beside its neighbours rather than inside itself, which is what the daemon's
		// copy does when it is given the directory that holds it
		"a file on its own": {path: "suelto.txt", want: "tres"},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			if got := e2e.Inside(t, session, project, "cat "+one.path); got != one.want {
				t.Errorf("cat %s inside = %q, want %q", one.path, got, one.want)
			}
		})
	}
}

// The two sides are already the same file there, and copying one onto the other is a way to lose
// whichever was newer.
func TestCopyingABindMountIsRefused(t *testing.T) {
	t.Parallel()

	session, project := mounted(t)
	project.Write(t, "montado/algo.txt", "montado\n")

	got := session.Run(t, project.Root, "copy-to-container", "montado")

	if got.Code != 6 {
		t.Fatalf("copy-to-container montado = %d, want 6\n%s", got.Code, got.Output())
	}

	if !strings.Contains(got.Output(), "bind mount") {
		t.Errorf("copy-to-container montado said %q, want it to say why", got.Output())
	}
}

// These are lists of paths a project may or may not have, and the caller is asking for whichever
// it does.
func TestCopyingSomethingThatIsNotThereIsSkipped(t *testing.T) {
	t.Parallel()

	session, project := mounted(t)
	project.Write(t, "app/code/uno.php", "uno\n")

	got := session.Run(t, project.Root, "copy-to-container", "no-existe", "app/code")
	if got.Code != 0 {
		t.Fatalf("copy-to-container no-existe app/code = %d, want 0\n%s", got.Code, got.Output())
	}

	if inside := e2e.Inside(t, session, project, "cat app/code/uno.php"); inside != "uno" {
		t.Errorf("cat app/code/uno.php inside = %q, want %q", inside, "uno")
	}
}

func TestCopyOutOfTheContainer(t *testing.T) {
	t.Parallel()

	session, project := mounted(t)

	e2e.Inside(t, session, project,
		"mkdir -p generated/code && printf 'generado\\n' > generated/code/hecho.php && "+
			"printf 'solo\\n' > salida.txt")

	session.Run(t, project.Root, "copy-from-container", "generated", "salida.txt")

	cases := map[string]struct{ path, want string }{
		"a directory":       {path: "generated/code/hecho.php", want: "generado"},
		"a file on its own": {path: "salida.txt", want: "solo"},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			if got := strings.TrimSpace(project.Read(one.path)); got != one.want {
				t.Errorf("%s on the host = %q, want %q", one.path, got, one.want)
			}
		})
	}
}

// Copying it again over what is already there replaces its contents rather than nesting a second
// copy inside the first, which is what a mirror has to do to be a mirror.
func TestCopyingOutAgainReplacesRatherThanNests(t *testing.T) {
	t.Parallel()

	session, project := mounted(t)

	e2e.Inside(t, session, project,
		"mkdir -p generated/code && printf 'generado\\n' > generated/code/hecho.php")
	session.Run(t, project.Root, "copy-from-container", "generated")

	e2e.Inside(t, session, project, "printf 'otro\\n' > generated/code/hecho.php")
	session.Run(t, project.Root, "copy-from-container", "generated")

	if got := strings.TrimSpace(project.Read("generated/code/hecho.php")); got != "otro" {
		t.Errorf("generated/code/hecho.php = %q, want %q", got, "otro")
	}

	if project.Has("generated/generated") {
		t.Error("generated/generated exists, want the copy to replace rather than nest")
	}
}

func TestCopyRefusals(t *testing.T) {
	t.Parallel()

	e2e.NeedsDocker(t)

	session := e2e.New(t)
	project := e2e.NewProject(t, "hm-e2e-copias-refus", e2e.Definition{})

	cases := map[string]struct {
		args []string
		want int
	}{
		"nothing to copy in":  {args: []string{"copy-to-container"}, want: 2},
		"nothing to copy out": {args: []string{"copy-from-container"}, want: 2},

		// The container being stopped is not something to find out from the daemon three layers
		// down
		"nothing running": {args: []string{"copy-to-container", "app"}, want: 5},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := session.Run(t, project.Root, one.args...); got.Code != one.want {
				t.Errorf("%v = %d, want %d\n%s", one.args, got.Code, one.want, got.Output())
			}
		})
	}
}
