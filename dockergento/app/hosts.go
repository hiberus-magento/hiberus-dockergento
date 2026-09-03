package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Hosts points a domain at this machine, and tells Magento what it answers on.
//
// Two different things, and they fail differently: one needs the system password and touches a
// file every program on the machine reads, and the other is a row in the project's database. Doing
// the first only when it is needed is the whole point — every entry costs a prompt and stays for
// as long as the machine lives.
type Hosts struct {
	Properties ports.Properties
	Resolver   ports.Resolver
	Legacy     ports.Legacy

	// File is the machine's hosts file. It is a field so that what this does to it can be seen by
	// a test, which is the only way anything about it was ever checked.
	File string

	Announce func(string)
	Binary   string
}

// Set records the domain and makes sure it reaches this machine.
func (h Hosts) Set(project core.Project, domain string, database bool) error {
	domain = trimmed(domain)

	if domain == "" {
		domain = project.Domain
	}

	if domain == "" {
		return core.Refusal{
			Kind:    "no_domain",
			Code:    2,
			Message: "There is no domain to set",
			Hint:    h.Binary + " set-host shop.test",
		}
	}

	if err := h.Properties.Set(project.Root, "DOMAIN", domain); err != nil {
		return err
	}

	if err := h.point(domain); err != nil {
		return err
	}

	if !database {
		return nil
	}

	h.say(fmt.Sprintf("Set https://%s/ to web/secure/base_url and web/unsecure/base_url.\n", domain))

	for _, path := range []string{"web/secure/base_url", "web/unsecure/base_url"} {
		code, err := h.Legacy.Run([]string{"magento", "config:set", path, "https://" + domain + "/"})
		if err != nil {
			return err
		}

		if code != 0 {
			return core.Refusal{
				Kind:    "config_failed",
				Code:    code,
				Message: "The base URL could not be written to the database",
				Hint:    h.Binary + " magento config:set " + path + " https://" + domain + "/",
			}
		}
	}

	return nil
}

// point adds the entry, when there is a reason to.
//
// A wildcard resolver for the TLD — ours, or one the machine already had — makes the entry
// pointless, and the entry is what costs a password prompt per project and leaves a line behind
// for ever. So: ask about the result, not about who produces it.
func (h Hosts) point(domain string) error {
	if h.Resolver != nil && h.Resolver.ResolvesLocally(domain) {
		h.say(fmt.Sprintf("%s already resolves to this machine, so %s was left alone.\n",
			domain, h.file()))

		return nil
	}

	contents, err := os.ReadFile(h.file())
	if err != nil {
		return err
	}

	if core.HostsHas(string(contents), domain) {
		return nil
	}

	h.say(fmt.Sprintf("Your system password is needed to add an entry to %s...\n", h.file()))

	return h.write(core.WithHost(string(contents), domain, h.Binary))
}

// Remove takes out what this tool added for a domain, and nothing else.
func (h Hosts) Remove(domain string) error {
	if domain == "" {
		return core.Refusal{
			Kind:    "no_domain",
			Code:    2,
			Message: "There is no domain to remove",
			Hint:    h.Binary + " set-host --remove shop.test",
		}
	}

	contents, err := os.ReadFile(h.file())
	if err != nil {
		return err
	}

	// A line somebody wrote by hand has no marker, so it is not ours to delete
	if !core.HostsAdded(string(contents), domain, h.Binary) {
		h.say(fmt.Sprintf("There is no entry for %s that %s added.\n", domain, h.Binary))

		return nil
	}

	h.say(fmt.Sprintf("Your system password is needed to remove the entry from %s...\n", h.file()))

	if err := h.write(core.WithoutHost(string(contents), domain, h.Binary)); err != nil {
		return err
	}

	h.say(fmt.Sprintf("Removed the entry for %s.\n", domain))

	return nil
}

// write puts the new contents in place.
//
// Copied in rather than moved: the hosts file has an owner, a mode and, on macOS, flags that a
// rename from a temporary directory would not carry. And through `sudo` only when it has to be —
// a file this user can write is written directly, which is what makes this testable at all.
func (h Hosts) write(contents string) error {
	if file, err := os.OpenFile(h.file(), os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		defer file.Close()

		_, err = file.WriteString(contents)

		return err
	}

	temporary, err := os.CreateTemp("", "hosts.*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())

	if _, err := temporary.WriteString(contents); err != nil {
		temporary.Close() //nolint:errcheck

		return err
	}

	if err := temporary.Close(); err != nil {
		return err
	}

	command := exec.Command("sudo", "cp", temporary.Name(), h.file())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}

func (h Hosts) file() string {
	if h.File != "" {
		return h.File
	}

	return "/etc/hosts"
}

func (h Hosts) say(message string) {
	if h.Announce != nil {
		h.Announce(message)
	}
}

// Resolution is how a domain reaches this machine, which is what a diagnosis reports.
func (h Hosts) Resolution(domain string) string {
	contents, err := os.ReadFile(h.file())
	if err == nil && core.HostsHas(string(contents), domain) {
		return "hosts"
	}

	if h.Resolver != nil && h.Resolver.ResolvesLocally(domain) {
		return "dns"
	}

	return "none"
}

// trimmed is a domain as somebody typed it: with the scheme and the path taken off, because
// `https://shop.test/` is a name with punctuation around it.
func trimmed(domain string) string {
	if at := strings.Index(domain, "://"); at >= 0 {
		domain = domain[at+3:]
	}

	if at := strings.Index(domain, "/"); at >= 0 {
		domain = domain[:at]
	}

	return domain
}
