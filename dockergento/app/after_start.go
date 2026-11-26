package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What a Linux environment needs once it is up, which is what post-start.sh, fix_linux_permissions.sh
// and set_etc_hosts.sh did and what `start`, `restart` and `rebuild` used to hand back to them.
//
// Two things. The code is bind mounted from the host, so the user PHP runs as inside the
// containers has to have the host user's ids, or whatever one side writes the other cannot touch.
// And the store calls itself by its own domains — an indexer, a cron, a full-page cache warmer —
// which only resolve inside the php container when /etc/hosts there sends them to the TLS
// terminator.
//
// The commands sent into the containers are the shell implementation's, byte for byte, quirks
// included: they are what every Linux environment has been fixed with until now, and a port is not
// the moment to change what they do.
//

// The services whose user and group are given the host's ids, in the order the shell did them.
var idServices = []string{"nginx", "phpfpm"}

// domainsQuery is what the domains of the store are read with, the shell's own statement.
const domainsQuery = "SELECT DISTINCT value FROM core_config_data WHERE path like 'web/%/base_url'"

// hostCertificates is where this machine keeps the certificate authorities it trusts locally, and
// what is copied into the php container when it is there.
const hostCertificates = "/usr/local/share/ca-certificates"

// AfterStart is what the platform needs once the environment is up.
//
// On macOS, nothing: the code lives in a volume there, so no ids have to match, and the domains
// reach the proxy through the host. On Linux it matches the ids and writes the self-routing
// entries. A failing step stops there with its error, as the shell's set -e did.
func (o Operator) AfterStart(project core.Project, files core.ComposeFiles) error {
	if o.Platform != "linux" {
		return nil
	}

	o.announce("Waiting for everything to spin up...\n")
	o.wait(5 * time.Second)

	o.announce("Fixing permissions\n")

	if err := o.matchIds(project, files); err != nil {
		return err
	}

	o.announce("Permissions fix finished\n")
	o.announce("Configuring self-routing domains...\n")

	return o.addSelfRoutingEntries(project, files)
}

// matchIds gives the user and the group PHP runs as, in both services, the host's ids.
func (o Operator) matchIds(project core.Project, files core.ComposeFiles) error {
	uid, gid := o.hostIds()

	for _, service := range idServices {
		if err := o.matchUserId(project, files, service, uid); err != nil {
			return err
		}

		if err := o.matchGroupId(project, files, service, gid); err != nil {
			return err
		}
	}

	return nil
}

func (o Operator) matchUserId(project core.Project, files core.ComposeFiles, service string, host int) error {
	current, err := o.readAsRoot(project, service, "getent passwd "+o.UserPHP+" | cut -f3 -d:")
	if err != nil {
		return err
	}

	// After reading the container's id and not before, which is the order the shell asked in: a
	// container that cannot be asked fails as that, whoever is running this
	if host == 0 {
		return core.Refusal{
			Kind:    "host_user_is_root",
			Code:    1,
			Message: "Something is wrong, HOST_UID cannot have id 0 (root)",
		}
	}

	wanted := strconv.Itoa(host)
	if current == wanted {
		return nil
	}

	o.announce(fmt.Sprintf("Changing UID of %s from %s to %s in %s service\n", o.UserPHP, current, wanted, service))

	if err := o.prepareToChangeIds(project, files, service); err != nil {
		return err
	}

	if err := o.runAsRoot(project, files, service, "usermod -u "+wanted+" -o "+o.UserPHP); err != nil {
		return err
	}

	return o.runAsRoot(project, files, service,
		"find / -xdev -user '"+current+"' -exec chown -h '"+o.UserPHP+"' {} \\;")
}

func (o Operator) matchGroupId(project core.Project, files core.ComposeFiles, service string, host int) error {
	current, err := o.readAsRoot(project, service, "getent group "+o.GroupPHP+" | cut -f3 -d:")
	if err != nil {
		return err
	}

	// Said and not stopped at, unlike the same case for the user: the shell printed this error and
	// carried on, having no exit after it, and that is the behaviour being ported
	if host == 0 {
		o.announce("Error: Something is wrong, HOST_UID cannot have id 0 (root).\n")
	}

	wanted := strconv.Itoa(host)
	if current == wanted {
		return nil
	}

	o.announce(fmt.Sprintf("Changing GID of %s from %s to %s in %s service\n", o.UserPHP, current, wanted, service))

	if err := o.prepareToChangeIds(project, files, service); err != nil {
		return err
	}

	if err := o.runAsRoot(project, files, service, "groupmod -g "+wanted+" -o "+o.GroupPHP); err != nil {
		return err
	}

	//
	// `-user` with the old group id, and not `-group`: the shell's own command, kept as it was.
	// It hands to the group the files whose owning user has that number, which is the same set
	// only where the user and the group of PHP were created with the same id — the usual case in
	// these images, and the reason nobody has noticed. Fixing it is a change of behaviour, and
	// that is not what a port is for.
	//
	return o.runAsRoot(project, files, service,
		"find / -xdev -user '"+current+"' -exec chgrp -h '"+o.UserPHP+"' {} \\;")
}

// prepareToChangeIds makes sure usermod and groupmod are there, installing them where the image
// is Alpine and refusing where it is not.
func (o Operator) prepareToChangeIds(project core.Project, files core.ComposeFiles, service string) error {
	hasUsermod, err := o.succeedsAsRoot(project, files, service, "type usermod >/dev/null 2>&1")
	if err != nil {
		return err
	}

	hasGroupmod := true

	if hasUsermod {
		if hasGroupmod, err = o.succeedsAsRoot(project, files, service, "type groupmod >/dev/null 2>&1"); err != nil {
			return err
		}
	}

	if hasUsermod && hasGroupmod {
		return nil
	}

	// "type apk /dev/null", without the redirection: the shell's own test, kept byte for byte
	hasApk, err := o.succeedsAsRoot(project, files, service, "type apk /dev/null 2>&1")
	if err != nil {
		return err
	}

	if !hasApk {
		return core.Refusal{
			Kind:    "usermod_missing",
			Code:    1,
			Message: "Commands usermod and groupmod are required",
		}
	}

	o.announce("Warning: installing shadow, this should be included in your image\n")

	return o.runAsRoot(project, files, service, "apk add --no-cache shadow")
}

// addSelfRoutingEntries writes into the php container's /etc/hosts an entry per domain of the
// store, and one for localhost, all pointing at the TLS terminator.
//
// A project routed through the global proxy has no terminator — the proxy overlay deletes Hitch,
// because the proxy terminates TLS itself — and there is nothing to point the entries at. Demanding
// it anyway is what made every Linux start of such a project fail after bringing it up, so it is
// skipped and said out loud.
func (o Operator) addSelfRoutingEntries(project core.Project, files core.ComposeFiles) error {
	php, err := resolveContainer(o.Engine, project, "phpfpm", o.Binary)
	if err != nil {
		return err
	}

	// Any failure to find it is the same as it not being there, as `|| true` made it in the shell
	hitch, err := resolveContainer(o.Engine, project, "hitch", o.Binary)

	if err != nil {
		o.announce("No TLS terminator in this project, so there are no self-routing entries to add\n")
	} else {
		if err := o.writeEntries(project, files, hitch); err != nil {
			return err
		}
	}

	return o.copyCertificates(php)
}

func (o Operator) writeEntries(project core.Project, files core.ComposeFiles, hitch string) error {
	address, err := o.Addresses.Address(hitch)
	if err != nil {
		return err
	}

	//
	// What the query says or fails with is not a reason to stop: the shell sent its errors to
	// /dev/null and looped over whatever came out, so a database that cannot be asked leaves the
	// container with the localhost entry only. Split on whitespace, the way the shell's `for` split
	// it, which is also what drops the column header — it has no "://" in it.
	//
	answer := &bytes.Buffer{}

	if o.Query != nil {
		_, _ = o.Query(project, domainsQuery, answer)
	}

	for _, value := range strings.Fields(answer.String()) {
		if !strings.Contains(value, "://") {
			continue
		}

		// The host goes into a bash -c run as root, and it comes from the store's configuration,
		// which an imported dump decides: anything but a host name is skipped, and a port is
		// dropped, since /etc/hosts has nowhere to put one
		host, _, _ := strings.Cut(hostOfBaseURL(value), ":")
		if !hostName.MatchString(host) {
			o.announce(fmt.Sprintf("Skipping %q: it is not a host name\n", value))

			continue
		}

		if err := o.appendHost(project, files, address, host); err != nil {
			return err
		}
	}

	return o.appendHost(project, files, address, "localhost")
}

func (o Operator) appendHost(project core.Project, files core.ComposeFiles, address, host string) error {
	command := []string{"bash", "-c", `echo "` + address + " " + host + `" >> /etc/hosts`}

	return o.mustSucceed(project, files, "phpfpm", command)
}

// copyCertificates gives the php container the certificate authorities this machine trusts
// locally, when it has any: the shell's set_etc_hosts.sh did it after the entries, and a store that
// calls itself over HTTPS needs to trust what signed its own certificate.
func (o Operator) copyCertificates(php string) error {
	if o.FS == nil || !o.FS.IsDir(hostCertificates) {
		return nil
	}

	if err := o.Files.Into(php, hostCertificates, "/usr/local/share/"); err != nil {
		return err
	}

	// Its output was sent to /dev/null, both streams; its failure was not tolerated
	status, err := o.RootRunner.Capture(php, []string{"update-ca-certificates"}, io.Discard, io.Discard)
	if err != nil {
		return err
	}

	if status != 0 {
		return fmt.Errorf("update-ca-certificates failed in phpfpm with exit code %d", status)
	}

	return nil
}

// hostOfBaseURL is what the shell's `sed -e 's|^[^/]*//||' -e 's|/.*$||'` leaves of a base URL: the
// scheme is dropped up to its two slashes, and everything from the next slash on.
func hostOfBaseURL(url string) string {
	if slash := strings.IndexByte(url, '/'); slash >= 0 && strings.HasPrefix(url[slash:], "//") {
		url = url[slash+2:]
	}

	if slash := strings.IndexByte(url, '/'); slash >= 0 {
		url = url[:slash]
	}

	return url
}

// readAsRoot is `$(compose exec -T -u root <service> sh -c "<command>")`: what the command wrote,
// with the trailing newlines a command substitution drops, and its failure as a failure.
func (o Operator) readAsRoot(project core.Project, service, command string) (string, error) {
	container, err := resolveContainer(o.Engine, project, service, o.Binary)
	if err != nil {
		return "", err
	}

	out := &bytes.Buffer{}

	status, err := o.RootRunner.Capture(container, []string{"sh", "-c", command}, out, o.errors())
	if err != nil {
		return "", err
	}

	if status != 0 {
		return "", fmt.Errorf("'%s' failed in %s with exit code %d", command, service, status)
	}

	return strings.TrimRight(out.String(), "\n"), nil
}

// runAsRoot is `compose exec -T -u root <service> sh -c "<command>"` under set -e.
func (o Operator) runAsRoot(project core.Project, files core.ComposeFiles, service, command string) error {
	return o.mustSucceed(project, files, service, []string{"sh", "-c", command})
}

// succeedsAsRoot is the same command asked as a question: its exit code is the answer, and only
// not reaching the container at all is an error.
func (o Operator) succeedsAsRoot(project core.Project, files core.ComposeFiles, service, command string) (bool, error) {
	status, err := o.Orchestrator.Exec(project, files, service, []string{"sh", "-c", command},
		core.ExecOptions{User: "root"})

	// The code before the error: the Compose library returns a non-zero exit as its code and an
	// error beside it, and that error is the answer "no", not a container out of reach
	if status != 0 {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (o Operator) mustSucceed(project core.Project, files core.ComposeFiles, service string, command []string) error {
	status, err := o.Orchestrator.Exec(project, files, service, command, core.ExecOptions{User: "root"})

	// The code before the error, for the same reason: the library's error beside a non-zero exit
	// has no message, and passing it on printed a blank line where this failure should be
	if status != 0 {
		return fmt.Errorf("'%s' failed in %s with exit code %d", strings.Join(command, " "), service, status)
	}

	return err
}

func (o Operator) wait(duration time.Duration) {
	if o.Wait != nil {
		o.Wait(duration)

		return
	}

	time.Sleep(duration)
}

func (o Operator) hostIds() (int, int) {
	if o.HostIds != nil {
		return o.HostIds()
	}

	return os.Getuid(), os.Getgid()
}

func (o Operator) errors() io.Writer {
	if o.Errors != nil {
		return o.Errors
	}

	return os.Stderr
}

// hostName is what may be written into /etc/hosts as a name: letters, digits, dots and hyphens.
var hostName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)
