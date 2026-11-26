package e2e_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/test/e2e"
)

//
// On Linux the code is bind mounted, so `start` gives the user PHP runs as, in nginx and phpfpm,
// the host's uid and gid. In 2.0.0-rc.8 that step stopped in silence on every host whose user was
// not 1000: asking an Alpine nginx whether it had usermod is a non-zero exit, the Compose library
// returns that exit with an error beside it, and the error was read first — as a container out of
// reach, with an empty message. The unit tests pin the order; this is the same path on a daemon.
//
// GitHub's Ubuntu runner is uid 1001, which is what makes it run there for real.
//

// The ids the fixture's `app` user is created with, which the start has to change.
const fixtureId = 1000

// Both services are Alpine without shadow, so each one goes through `apk add shadow` before usermod
// and groupmod, the branch rc.8 never reached. The user is created as the container starts, which
// keeps the pull to one small image instead of a build.
//
// phpfpm also gets an update-ca-certificates that does nothing: when the host has
// /usr/local/share/ca-certificates (Ubuntu does) the start copies it in and runs that command, and
// plain Alpine does not have it. What it would do is not what this test is about.
const permissionsServices = `  nginx:
    image: alpine:latest
    command: ["sh", "-c", "addgroup -g 1000 app && adduser -D -u 1000 -G app app && exec sleep 600"]
  phpfpm:
    image: alpine:latest
    working_dir: /var/www/html
    command: ["sh", "-c", "addgroup -g 1000 app && adduser -D -u 1000 -G app app && printf '#!/bin/sh\\nexit 0\\n' > /usr/local/bin/update-ca-certificates && chmod +x /usr/local/bin/update-ca-certificates && exec sleep 600"]
`

func TestStartGivesAppTheHostIdsOnLinux(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("skipping: the ids are only matched on Linux, where the code is bind mounted")
	}

	uid, gid := os.Getuid(), os.Getgid()

	if uid == 0 {
		t.Skip("skipping: run as root, which start refuses before changing anything")
	}

	// Nothing would change, and a test that changes nothing passes whether the step works or not
	if uid == fixtureId && gid == fixtureId {
		t.Skipf("skipping: this host is uid %d and gid %d, the ids the fixture already has, so "+
			"there is nothing to change; it runs for real on a host that is not 1000 (GitHub's "+
			"Ubuntu runner is 1001)", uid, gid)
	}

	e2e.NeedsDocker(t)

	session := e2e.New(t)
	project := e2e.NewProject(t, "hm-e2e-ids", e2e.Definition{Services: permissionsServices})

	// Registered before the start, so that a start that fails halfway is cleaned up too
	t.Cleanup(func() {
		e2e.Compose(t, session, project, "down", "-v", "--remove-orphans")
	})

	result := session.Run(t, project.Root, "--no-json", "start")

	if result.Code != 0 {
		t.Fatalf("start = %d, want 0\n%s", result.Code, result.Output())
	}

	if !strings.Contains(result.Output(), "Permissions fix finished") {
		t.Fatalf("start did not finish fixing permissions, want \"Permissions fix finished\"\n%s",
			result.Output())
	}

	for _, service := range []string{"nginx", "phpfpm"} {
		for _, id := range []struct {
			flag string
			want int
		}{
			{"-u", uid},
			{"-g", gid},
		} {
			got := e2e.InsideService(t, session, project, service, "id "+id.flag+" app")

			if got != strconv.Itoa(id.want) {
				t.Errorf("id %s app in %s = %q, want %d\nstart said:\n%s",
					id.flag, service, got, id.want, result.Output())
			}
		}
	}
}
