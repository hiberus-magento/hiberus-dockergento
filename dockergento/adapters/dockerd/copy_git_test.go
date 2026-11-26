package dockerd

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

//
// A whole-tree copy never carries the repository: the reported case was a copy back out of the
// container that wrote a stale .git over the host's, failing on git's read-only objects halfway.
//

// checkout makes a project with a repository at its top — the directory, and a worktree's .git
// file beside it under another root — plus a package Composer installed from source, whose own
// .git is part of the dependency and not the project's repository.
func checkout(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "shop")

	for name, content := range map[string]string{
		".git/HEAD":                   "ref: refs/heads/main\n",
		".git/objects/00/1c2e":        "object",
		"composer.json":               "{}",
		"app/etc/env.php":             "<?php",
		"vendor/acme/pkg/.git/HEAD":   "ref: refs/heads/1.x\n",
		"vendor/acme/pkg/src/One.php": "<?php",
		".gitignore":                  "/vendor/",
	} {
		path := filepath.Join(root, name)

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// names lists the entries of a tar stream, sorted.
func names(t *testing.T, stream io.Reader) []string {
	t.Helper()

	var found []string

	archive := tar.NewReader(stream)

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		found = append(found, header.Name)
	}

	sort.Strings(found)

	return found
}

// archived builds a tar stream from name → contents, a name ending in / being a directory.
func archived(t *testing.T, entries [][2]string) *bytes.Buffer {
	t.Helper()

	var stream bytes.Buffer

	archive := tar.NewWriter(&stream)

	for _, entry := range entries {
		header := &tar.Header{Name: entry[0], Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(entry[1]))}

		if strings.HasSuffix(entry[0], "/") || entry[0] == "." {
			header = &tar.Header{Name: entry[0], Mode: 0o755, Typeflag: tar.TypeDir}
		}

		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}

		if header.Typeflag == tar.TypeReg {
			if _, err := archive.Write([]byte(entry[1])); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	return &stream
}

func hasGitAtTop(found []string, top string) bool {
	for _, name := range found {
		under := strings.TrimPrefix(strings.TrimPrefix(name, top+"/"), "./")
		if under == ".git" || strings.HasPrefix(under, ".git/") {
			return true
		}
	}

	return false
}

func TestPackingATreeLeavesItsRepositoryOut(t *testing.T) {
	cases := []struct {
		name   string
		source func(root string) string
	}{
		{"the directory itself", func(root string) string { return root }},
		{"its contents, as copy-to-container --all names them", func(root string) string { return root + "/." }},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root := checkout(t)

			var stream bytes.Buffer
			if err := pack(&stream, tt.source(root), nil); err != nil {
				t.Fatal(err)
			}

			found := names(t, &stream)

			if hasGitAtTop(found, "shop") {
				t.Fatalf("packed %v, want no .git at the top of the tree", found)
			}

			for _, kept := range []string{"/.gitignore", "/composer.json", "/vendor/acme/pkg/.git/HEAD"} {
				if !endsWith(found, kept) {
					t.Errorf("packed %v, want it to keep %s", found, kept)
				}
			}
		})
	}
}

func TestPackingAWorktreeLeavesItsGitFileOut(t *testing.T) {
	root := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string]string{".git": "gitdir: /code/shop/.git/worktrees/wt\n", "index.php": "<?php"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stream bytes.Buffer
	if err := pack(&stream, root, nil); err != nil {
		t.Fatal(err)
	}

	found := names(t, &stream)
	if want := []string{"wt", "wt/index.php"}; strings.Join(found, ",") != strings.Join(want, ",") {
		t.Fatalf("packed %v, want %v", found, want)
	}
}

func TestUnpackingATreeNeverWritesIntoTheRepository(t *testing.T) {
	cases := []struct {
		name    string
		top     string
		entries [][2]string
	}{
		{"the daemon's form for a copy of contents (workdir/.)", ".", [][2]string{
			{".", ""}, {"./.git/", ""}, {"./.git/objects/00/1c2e", "stale"}, {"./composer.json", "new"},
			{"./vendor/acme/pkg/.git/HEAD", "pkg"},
		}},
		{"the daemon's form for a named directory", "html", [][2]string{
			{"html/", ""}, {"html/.git/objects/00/1c2e", "stale"}, {"html/composer.json", "new"},
			{"html/vendor/acme/pkg/.git/HEAD", "pkg"},
		}},
		{"a worktree's .git file", ".", [][2]string{{"./.git", "gitdir: elsewhere"}, {"./composer.json", "new"},
			{"./vendor/acme/pkg/.git/HEAD", "pkg"}}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// A named directory lands on the host's directory of the same name, as From aims it
			target := filepath.Join(t.TempDir(), "shop")
			if tt.top != "." {
				target = filepath.Join(filepath.Dir(target), tt.top)
			}

			repository := filepath.Join(target, ".git", "objects", "00")

			if err := os.MkdirAll(repository, 0o755); err != nil {
				t.Fatal(err)
			}

			object := filepath.Join(repository, "1c2e")
			if err := os.WriteFile(object, []byte("host"), 0o444); err != nil {
				t.Fatal(err)
			}

			if err := unpack(archived(t, tt.entries), target, tt.top, nil); err != nil {
				t.Fatalf("unpack: %v", err)
			}

			if got, _ := os.ReadFile(object); string(got) != "host" {
				t.Fatalf("the host's object holds %q, want it untouched", got)
			}

			if got, _ := os.ReadFile(filepath.Join(target, "composer.json")); string(got) != "new" {
				t.Fatalf("composer.json = %q, want the copied one", got)
			}

			// A package installed from source keeps its own repository: it is the dependency
			if got, _ := os.ReadFile(filepath.Join(target, "vendor", "acme", "pkg", ".git", "HEAD")); string(got) != "pkg" {
				t.Fatalf("vendor/acme/pkg/.git/HEAD = %q, want it copied", got)
			}
		})
	}
}

// daemon poses as the Docker daemon for the copy endpoints: it serves out as the archive of any
// path, and keeps what is put.
type daemon struct {
	out []byte

	mu  sync.Mutex
	put []byte
}

func (d *daemon) serve(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if i := strings.Index(path, "/containers/"); i > 0 {
			path = path[i:]
		}

		switch {
		case strings.HasSuffix(path, "/_ping"):
			w.Header().Set("Api-Version", "1.43")
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(path, "/archive") && r.Method == http.MethodGet:
			w.Header().Set("X-Docker-Container-Path-Stat",
				base64.StdEncoding.EncodeToString([]byte(`{"name":"html","mode":2147484141}`)))
			w.Write(d.out) //nolint:errcheck
		case strings.HasSuffix(path, "/archive") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			d.mu.Lock()
			d.put = body
			d.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_API_VERSION", "")
}

func TestCopyingTheWorkdirOutLeavesTheHostRepositoryAlone(t *testing.T) {
	fake := &daemon{out: archived(t, [][2]string{
		{".", ""}, {"./.git/objects/00/1c2e", "stale"}, {"./composer.json", "new"},
	}).Bytes()}
	fake.serve(t)

	target := filepath.Join(t.TempDir(), "shop")
	if err := os.MkdirAll(filepath.Join(target, ".git", "objects", "00"), 0o755); err != nil {
		t.Fatal(err)
	}

	object := filepath.Join(target, ".git", "objects", "00", "1c2e")
	if err := os.WriteFile(object, []byte("host"), 0o444); err != nil {
		t.Fatal(err)
	}

	if err := (Files{}).From("abc123", "/var/www/html/.", target); err != nil {
		t.Fatalf("From: %v", err)
	}

	if got, _ := os.ReadFile(object); string(got) != "host" {
		t.Fatalf("the host's object holds %q, want it untouched", got)
	}

	// The positive control: the fake did serve the tree, and the rest of it landed
	if got, _ := os.ReadFile(filepath.Join(target, "composer.json")); string(got) != "new" {
		t.Fatalf("composer.json = %q, want the copied one", got)
	}
}

func TestCopyingEverythingInSendsNoRepository(t *testing.T) {
	fake := &daemon{}
	fake.serve(t)

	root := checkout(t)

	if err := (Files{}).Into("abc123", root+"/.", "/var/www/html"); err != nil {
		t.Fatalf("Into: %v", err)
	}

	fake.mu.Lock()
	found := names(t, bytes.NewReader(fake.put))
	fake.mu.Unlock()

	// The contents of the project, named as they land in the workdir, not under the project's name
	if !slices.Contains(found, "composer.json") {
		t.Fatalf("sent %v, want the project's composer.json at the top", found)
	}

	if hasGitAtTop(found, "shop") {
		t.Fatalf("sent %v, want no .git at the top of the tree", found)
	}
}

// endsWith is whether a name ends at a path boundary with wanted ("/x"): "shop/x" and "x" both do.
func endsWith(values []string, wanted string) bool {
	for _, value := range values {
		if strings.HasSuffix("/"+value, wanted) {
			return true
		}
	}

	return false
}

// "dir/." sends what is in dir, the way `docker cp dir/. container:/target` does: copy-to-container
// --all extracts it into the workdir, so names carrying the project directory's own name would
// land in workdir/<project>/ instead.
func TestPackingTheContentsOfADirectorySendsNamesWithoutIt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "commerce")
	if err := os.MkdirAll(filepath.Join(root, "app", "etc"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "app", "etc", "env.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}

	var contents, itself bytes.Buffer

	if err := pack(&contents, root+string(filepath.Separator)+".", nil); err != nil {
		t.Fatal(err)
	}

	if err := pack(&itself, root, nil); err != nil {
		t.Fatal(err)
	}

	for _, name := range names(t, &contents) {
		if strings.HasPrefix(strings.TrimPrefix(name, "./"), "commerce") {
			t.Fatalf("packing %s/. sent %q: the contents carry the directory's own name", root, name)
		}
	}

	found := false
	for _, name := range names(t, &itself) {
		if name == "commerce/app/etc/env.php" {
			found = true
		}
	}

	if !found {
		t.Fatalf("packing %s itself no longer sends commerce/…: %v", root, names(t, &itself))
	}
}
