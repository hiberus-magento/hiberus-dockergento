package cli

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What purge, npm, n98-magerun and the two test suites ask of the engine — all five reach the
// container only through inside (php.go:57), so what they differ in is the command they hand it,
// not how they reach it. want values come from the parity suite (tests/integration/go_wrappers_
// test.sh), never from output observed by running the code under test.
//

// TestWhatPurgeAsks asserts the seven directories literally, not reassembled from `generated`
// through strings.Join: a typo in the production list and a matching typo here would both pass.
func TestWhatPurgeAsks(t *testing.T) {
	root := "/code/shop"

	fake := answering(t)
	fake.project = core.Project{Name: "shop", Root: root}
	cwd := here()

	code := purge(io.Discard, io.Discard, false)

	if code != exitOK {
		t.Fatalf("purge = %d, want exitOK", code)
	}

	want := []call{
		{Method: "Resolve", Dir: cwd},
		{Method: "Exec", Dir: root, Service: phpService,
			Command: []string{"sh", "-c",
				"rm -rf var/cache/* generated/* pub/static/* var/view_preprocessed/* " +
					"var/page_cache/* var/generation/* dev/tests/integration/tmp/*"},
			Options: terminalOptions("")},
	}

	if diff := cmp.Diff(want, fake.calls); diff != "" {
		t.Fatalf("asked of the engine (-want +got):\n%s", diff)
	}
}

// TestWhatNpmAndMagerunAsk is the one place that says the two look-alike passthroughs are not
// alike at all: npm's arguments reach exec's own argv, n98-magerun's are joined through a shell.
func TestWhatNpmAndMagerunAsk(t *testing.T) {
	root := "/code/shop"

	t.Run("npm passes its arguments straight through", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		cwd := here()

		code := npm([]string{"run", "build"}, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("npm = %d, want exitOK", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"npm", "run", "build"}, Options: terminalOptions("")},
		}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})

	// n98-magerun's own argument parsing is looser than exec's argv, which is why the shell
	// implementation joined the arguments into one string instead of passing them separately.
	t.Run("n98-magerun joins its arguments through a shell", func(t *testing.T) {
		fake := answering(t)
		fake.project = core.Project{Name: "shop", Root: root}
		cwd := here()

		code := magerun([]string{"cache:report", "--format=json"}, io.Discard, io.Discard, false)

		if code != exitOK {
			t.Fatalf("n98-magerun = %d, want exitOK", code)
		}

		want := []call{
			{Method: "Resolve", Dir: cwd},
			{Method: "Exec", Dir: root, Service: phpService,
				Command: []string{"bash", "-c", "n98-magerun cache:report --format=json"},
				Options: terminalOptions("")},
		}

		if diff := cmp.Diff(want, fake.calls); diff != "" {
			t.Fatalf("asked of the engine (-want +got):\n%s", diff)
		}
	})
}

// TestWhatTheTestSuitesAsk is a table over the two properties tests() reads, and their fallbacks:
// a project that never set BIN_DIR still gets ./vendor/bin/phpunit, and one that never set
// WORKDIR_PHP still gets /var/www/html — composed with the fallback BIN_DIR into a doubled "/./"
// that is pinned here, not fixed, because that is what wrappers.go actually asks Docker to run.
func TestWhatTheTestSuitesAsk(t *testing.T) {
	root := "/code/shop"

	cases := []struct {
		name       string
		kind       string
		properties map[string]string
		args       []string
		want       func(cwd string) []call
	}{
		{
			name:       "unit suite with a bin directory configured",
			kind:       "unit",
			properties: map[string]string{"BIN_DIR": "bin"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c", "bin/phpunit --config ./dev/tests/unit/phpunit.xml.dist"},
						Options: terminalOptions("")},
				}
			},
		},
		{
			name: "unit suite falls back to ./vendor/bin",
			kind: "unit",
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c", "./vendor/bin/phpunit --config ./dev/tests/unit/phpunit.xml.dist"},
						Options: terminalOptions("")},
				}
			},
		},
		{
			name:       "unit suite with appended arguments",
			kind:       "unit",
			properties: map[string]string{"BIN_DIR": "bin"},
			args:       []string{"--filter", "Checkout"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c",
							"bin/phpunit --config ./dev/tests/unit/phpunit.xml.dist --filter Checkout"},
						Options: terminalOptions("")},
				}
			},
		},
		{
			name:       "integration suite with both properties configured",
			kind:       "integration",
			properties: map[string]string{"BIN_DIR": "bin", "WORKDIR_PHP": "/app"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Property", Dir: root, Key: "WORKDIR_PHP"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c", "cd ./dev/tests/integration && /app/bin/phpunit --config phpunit.xml"},
						Options: terminalOptions("")},
				}
			},
		},
		{
			// Neither property set: WORKDIR_PHP falls back to /var/www/html, BIN_DIR to
			// ./vendor/bin, and the two fallbacks compose into "/var/www/html/./vendor/bin" —
			// the doubled "/./" the open question in design.md leaves unfixed.
			name: "integration suite falls back on both properties, doubled slash and all",
			kind: "integration",
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Property", Dir: root, Key: "WORKDIR_PHP"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c",
							"cd ./dev/tests/integration && /var/www/html/./vendor/bin/phpunit --config phpunit.xml"},
						Options: terminalOptions("")},
				}
			},
		},
		{
			name:       "integration suite with appended arguments",
			kind:       "integration",
			properties: map[string]string{"BIN_DIR": "bin", "WORKDIR_PHP": "/app"},
			args:       []string{"--group", "checkout"},
			want: func(cwd string) []call {
				return []call{
					{Method: "Resolve", Dir: cwd},
					{Method: "Property", Dir: root, Key: "BIN_DIR"},
					{Method: "Property", Dir: root, Key: "WORKDIR_PHP"},
					{Method: "Exec", Dir: root, Service: phpService,
						Command: []string{"sh", "-c",
							"cd ./dev/tests/integration && /app/bin/phpunit --config phpunit.xml --group checkout"},
						Options: terminalOptions("")},
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := answering(t)
			fake.project = core.Project{Name: "shop", Root: root}
			fake.properties = tt.properties
			cwd := here()

			code := tests(tt.kind, tt.args, io.Discard, io.Discard, false)

			if code != exitOK {
				t.Fatalf("test-%s = %d, want exitOK", tt.kind, code)
			}

			if diff := cmp.Diff(tt.want(cwd), fake.calls); diff != "" {
				t.Fatalf("asked of the engine (-want +got):\n%s", diff)
			}
		})
	}
}
