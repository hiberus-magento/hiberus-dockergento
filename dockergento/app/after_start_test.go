package app

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What a Linux environment gets once it is up: the host's ids for PHP's user and group in nginx and
// phpfpm, and the self-routing entries in phpfpm's /etc/hosts. Every container command lands in
// one ordered log, shared by the orchestrator (what Compose runs as root), the root runner (what is
// read back) and the rest, so each test compares a single sequence — the commands are the shell's,
// byte for byte, and a drift in any of them is a diff here.
//

var linuxProject = core.Project{Name: "tienda", Root: "/code/tienda"}

// execEntry is how the orchestrator fake logs an exec, and captureEntry how the root runner logs
// what it reads.
func execEntry(user, service string, command ...string) string {
	return fmt.Sprintf("exec -u %s %s %q", user, service, command)
}

func captureEntry(container string, command ...string) string {
	return fmt.Sprintf("capture %s %q", container, command)
}

func containsEntry(log []string, wanted string) bool {
	for _, entry := range log {
		if entry == wanted {
			return true
		}
	}

	return false
}

// rootRunner answers a capture by the command it ran: answers is what it writes, and statuses its
// exit code. A command it does not name writes nothing and exits 0.
type rootRunner struct {
	log      *[]string
	answers  map[string]string
	statuses map[string]int
}

func (r *rootRunner) Run(container string, command, _ []string, _ io.Writer) (int, error) {
	*r.log = append(*r.log, "run "+container+" "+strings.Join(command, " "))

	return 0, nil
}

func (r *rootRunner) Feed(container string, command []string, _ io.Reader, _ io.Writer) (int, error) {
	*r.log = append(*r.log, "feed "+container+" "+strings.Join(command, " "))

	return 0, nil
}

func (r *rootRunner) Capture(container string, command []string, out, _ io.Writer) (int, error) {
	*r.log = append(*r.log, captureEntry(container, command...))
	last := command[len(command)-1]
	fmt.Fprint(out, r.answers[last])

	return r.statuses[last], nil
}

type addresses struct {
	log     *[]string
	address string
}

func (a addresses) Address(container string) (string, error) {
	*a.log = append(*a.log, "address "+container)

	return a.address, nil
}

// The base URLs as the client prints them: a header, then one per line. One plain, one secure with
// a path, which is what the host has to be cut out of.
const twoBaseURLs = "value\nhttp://tienda.local/\nhttps://secure.tienda.local/es/tienda\n"

func runningLinuxStack(services ...string) []core.Container {
	containers := []core.Container{}

	for _, service := range services {
		containers = append(containers, core.Container{
			ID: service + "-1", Running: true, ComposeProject: "tienda", ComposeService: service,
		})
	}

	return containers
}

// linuxOperator is an operator on Linux over a running nginx, phpfpm and hitch, whose PHP user is
// app and group www, where the containers answer uid and gid with what they are given and the host
// is 1000:1000. It waits by logging, and its database answers the two base URLs.
func linuxOperator(log *[]string, orchestration *orchestrator, legacy *shell, uid, gid string, announced *[]string) Operator {
	operator := operatorWith(runningLinuxStack("nginx", "phpfpm", "hitch"), orchestration, legacy)
	operator.Platform = "linux"
	operator.UserPHP = "app"
	operator.GroupPHP = "www"
	operator.RootRunner = &rootRunner{log: log, answers: map[string]string{
		"getent passwd app | cut -f3 -d:": uid + "\n",
		"getent group www | cut -f3 -d:":  gid + "\n",
	}}
	operator.Addresses = addresses{log: log, address: "172.18.0.9"}
	operator.Files = &files{}
	operator.FS = filesystem{}
	operator.Wait = func(duration time.Duration) { *log = append(*log, "wait "+duration.String()) }
	operator.HostIds = func() (int, int) { return 1000, 1000 }
	operator.Query = func(_ core.Project, statement string, out io.Writer) (int, error) {
		*log = append(*log, "query "+statement)
		fmt.Fprint(out, twoBaseURLs)

		return 0, nil
	}

	if announced != nil {
		operator.Announce = func(message string) { *announced = append(*announced, message) }
	}

	return operator
}

// The ids read from both services, when nothing has to change.
func idsRead() []string {
	return []string{
		captureEntry("nginx-1", "sh", "-c", "getent passwd app | cut -f3 -d:"),
		captureEntry("nginx-1", "sh", "-c", "getent group www | cut -f3 -d:"),
		captureEntry("phpfpm-1", "sh", "-c", "getent passwd app | cut -f3 -d:"),
		captureEntry("phpfpm-1", "sh", "-c", "getent group www | cut -f3 -d:"),
	}
}

func hostsEntry(line string) string {
	return execEntry("root", "phpfpm", "bash", "-c", `echo "`+line+`" >> /etc/hosts`)
}

// The self-routing entries for the two base URLs, then localhost.
func selfRouting() []string {
	return []string{
		"address hitch-1",
		"query " + domainsQuery,
		hostsEntry("172.18.0.9 tienda.local"),
		hostsEntry("172.18.0.9 secure.tienda.local"),
		hostsEntry("172.18.0.9 localhost"),
	}
}

func TestDifferentIdsAreGivenTheHostsInBothServices(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "33", "82", nil)

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	want := []string{"wait 5s"}

	for _, service := range []string{"nginx", "phpfpm"} {
		want = append(want,
			captureEntry(service+"-1", "sh", "-c", "getent passwd app | cut -f3 -d:"),
			execEntry("root", service, "sh", "-c", "type usermod >/dev/null 2>&1"),
			execEntry("root", service, "sh", "-c", "type groupmod >/dev/null 2>&1"),
			execEntry("root", service, "sh", "-c", "usermod -u 1000 -o app"),
			execEntry("root", service, "sh", "-c", `find / -xdev -user '33' -exec chown -h 'app' {} \;`),
			captureEntry(service+"-1", "sh", "-c", "getent group www | cut -f3 -d:"),
			execEntry("root", service, "sh", "-c", "type usermod >/dev/null 2>&1"),
			execEntry("root", service, "sh", "-c", "type groupmod >/dev/null 2>&1"),
			execEntry("root", service, "sh", "-c", "groupmod -g 1000 -o www"),
			// -user with the old gid and chgrp to the user: the shell's own command, quirk kept
			execEntry("root", service, "sh", "-c", `find / -xdev -user '82' -exec chgrp -h 'app' {} \;`),
		)
	}

	want = append(want, selfRouting()...)

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}
}

func TestMatchingIdsChangeNothing(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "1000", "1000", nil)

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	want := append(append([]string{"wait 5s"}, idsRead()...), selfRouting()...)

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}
}

func TestARootHostUserIsRefused(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "33", "33", nil)
	operator.HostIds = func() (int, int) { return 0, 0 }

	refusal := refusalOf(t, operator.AfterStart(linuxProject, core.ComposeFiles{}))

	if refusal.Kind != "host_user_is_root" || refusal.Code != 1 ||
		refusal.Message != "Something is wrong, HOST_UID cannot have id 0 (root)" {
		t.Fatalf("refusal = %+v, want host_user_is_root with code 1 and the shell's sentence", refusal)
	}

	want := []string{"wait 5s", captureEntry("nginx-1", "sh", "-c", "getent passwd app | cut -f3 -d:")}

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}
}

func TestWithoutUsermodAndWithoutApkItStops(t *testing.T) {
	log := []string{}
	orchestration := &orchestrator{log: &log, exits: map[string]int{
		"type usermod >/dev/null 2>&1": 1,
		"type apk /dev/null 2>&1":      1,
	}}
	operator := linuxOperator(&log, orchestration, &shell{}, "33", "33", nil)

	refusal := refusalOf(t, operator.AfterStart(linuxProject, core.ComposeFiles{}))

	if refusal.Kind != "usermod_missing" || refusal.Code != 1 {
		t.Fatalf("refusal = %+v, want usermod_missing with code 1", refusal)
	}

	// groupmod is not asked about once usermod is missing, as the shell's `||` did not ask
	want := []string{
		"wait 5s",
		captureEntry("nginx-1", "sh", "-c", "getent passwd app | cut -f3 -d:"),
		execEntry("root", "nginx", "sh", "-c", "type usermod >/dev/null 2>&1"),
		execEntry("root", "nginx", "sh", "-c", "type apk /dev/null 2>&1"),
	}

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}
}

func TestOnAlpineShadowIsInstalledFirst(t *testing.T) {
	log, announced := []string{}, []string{}
	orchestration := &orchestrator{log: &log, exits: map[string]int{"type groupmod >/dev/null 2>&1": 1}}
	operator := linuxOperator(&log, orchestration, &shell{}, "33", "1000", &announced)

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	want := []string{
		"wait 5s",
		captureEntry("nginx-1", "sh", "-c", "getent passwd app | cut -f3 -d:"),
		execEntry("root", "nginx", "sh", "-c", "type usermod >/dev/null 2>&1"),
		execEntry("root", "nginx", "sh", "-c", "type groupmod >/dev/null 2>&1"),
		execEntry("root", "nginx", "sh", "-c", "type apk /dev/null 2>&1"),
		execEntry("root", "nginx", "sh", "-c", "apk add --no-cache shadow"),
		execEntry("root", "nginx", "sh", "-c", "usermod -u 1000 -o app"),
	}

	if diff := cmp.Diff(want, log[:len(want)]); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}

	if !containsEntry(announced, "Warning: installing shadow, this should be included in your image\n") {
		t.Fatalf("announced = %q, want the warning about shadow", announced)
	}
}

func TestAFailingChangeStopsThere(t *testing.T) {
	// set -e: a usermod that fails is not followed by the find that would hand the files over
	log := []string{}
	orchestration := &orchestrator{log: &log, exits: map[string]int{"usermod -u 1000 -o app": 1}}
	operator := linuxOperator(&log, orchestration, &shell{}, "33", "33", nil)

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err == nil {
		t.Fatal("AfterStart with a failing usermod = nil, want the failure")
	}

	if last := log[len(log)-1]; last != execEntry("root", "nginx", "sh", "-c", "usermod -u 1000 -o app") {
		t.Fatalf("last command = %q, want the usermod that failed", last)
	}
}

func TestWithoutATLSTerminatorThereAreNoEntries(t *testing.T) {
	log, announced := []string{}, []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "1000", "1000", &announced)
	operator.Engine = &engine{containers: runningLinuxStack("nginx", "phpfpm")}

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	want := append([]string{"wait 5s"}, idsRead()...)

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}

	if !containsEntry(announced, "No TLS terminator in this project, so there are no self-routing entries to add\n") {
		t.Fatalf("announced = %q, want the announcement that there is nothing to add", announced)
	}
}

func TestAFailingQueryAddsOnlyLocalhost(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "1000", "1000", nil)
	operator.Query = func(core.Project, string, io.Writer) (int, error) {
		return 0, errors.New("the database is not running")
	}

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want the query's failure tolerated", err)
	}

	want := append(append([]string{"wait 5s"}, idsRead()...),
		"address hitch-1", hostsEntry("172.18.0.9 localhost"))

	if diff := cmp.Diff(want, log); diff != "" {
		t.Fatalf("container commands (-want +got):\n%s", diff)
	}
}

func TestTheHostsCertificateAuthoritiesAreCopiedIn(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "1000", "1000", nil)
	copies := &files{}
	operator.Files = copies
	operator.FS = filesystem{present: map[string]bool{"/usr/local/share/ca-certificates": true}}

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	if diff := cmp.Diff([]string{"/usr/local/share/"}, copies.into); diff != "" {
		t.Fatalf("copied into (-want +got):\n%s", diff)
	}

	if last := log[len(log)-1]; last != captureEntry("phpfpm-1", "update-ca-certificates") {
		t.Fatalf("last command = %q, want the certificates updated in phpfpm", last)
	}
}

func TestOnMacOSThereIsNothingToDoAfterStarting(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "33", "33", nil)
	operator.Platform = "mac"

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart on macOS = %v, want no error", err)
	}

	if len(log) != 0 {
		t.Fatalf("done on macOS = %v, want nothing at all", log)
	}
}

func TestTheHostIsCutOutOfABaseURL(t *testing.T) {
	cases := map[string]string{
		"http://tienda.local/":                  "tienda.local",
		"https://secure.tienda.local/es/tienda": "secure.tienda.local",
		"https://tienda.local:8443":             "tienda.local:8443",
		"{{unsecure_base_url}}://x":             "x",
	}

	for url, want := range cases {
		if got := hostOfBaseURL(url); got != want {
			t.Errorf("hostOfBaseURL(%q) = %q, want %q", url, got, want)
		}
	}
}

// TestABaseURLThatIsNotAHostNameIsNotWritten: the hosts come out of the store's configuration,
// which an imported dump decides, and go into a `bash -c` run as root. Anything but a host name is
// skipped; a port is dropped, since /etc/hosts has no place for one.
func TestABaseURLThatIsNotAHostNameIsNotWritten(t *testing.T) {
	log := []string{}
	operator := linuxOperator(&log, &orchestrator{log: &log}, &shell{}, "1000", "1000", nil)
	operator.Query = func(_ core.Project, statement string, out io.Writer) (int, error) {
		log = append(log, "query "+statement)
		fmt.Fprint(out, "value\nhttps://$(touch${IFS}/pwned).test/\nhttps://tienda.local:8443/\n")

		return 0, nil
	}

	if err := operator.AfterStart(linuxProject, core.ComposeFiles{}); err != nil {
		t.Fatalf("AfterStart = %v, want no error", err)
	}

	written := []string{}
	for _, entry := range log {
		if strings.Contains(entry, "/etc/hosts") {
			written = append(written, entry)
		}
	}

	want := []string{hostsEntry("172.18.0.9 tienda.local"), hostsEntry("172.18.0.9 localhost")}
	if diff := cmp.Diff(want, written); diff != "" {
		t.Fatalf("written to /etc/hosts (-want +got):\n%s", diff)
	}
}
