// Package machine answers what the host says about itself, as opposed to what Docker says.
//
// The distinction is the whole point of one of the checks: on macOS the containers do not run on
// the laptop, they run in a virtual machine with whatever memory somebody gave it once, and the
// two numbers being different is what nobody was told.
package machine

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"syscall"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// Host is this machine.
type Host struct{}

// MemoryBytes is the machine's own memory.
func (Host) MemoryBytes() int64 { return memoryBytes() }

// FreeDiskGB is the space left on the startup disk, and whether it could be read at all.
//
// Truncated rather than rounded, because `df` truncates and this number is checked against `df`
// by whoever does not believe it.
func (Host) FreeDiskGB() (int, bool) {
	var stat syscall.Statfs_t

	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, false
	}

	return int(uint64(stat.Bavail) * uint64(stat.Bsize) / (1024 * 1024 * 1024)), true
}

// ErrNoPortTool is returned when nothing on this machine can list the ports in use. It is a
// finding of its own rather than a silent pass: a check that cannot look must not report that
// everything is free.
var ErrNoPortTool = errNoPortTool{}

type errNoPortTool struct{}

func (errNoPortTool) Error() string { return "no tool available to inspect listening ports" }

var listeningPort = regexp.MustCompile(`[:.](\d+)$`)

// Listening is every port held on this machine, with the process holding it when the tool that
// listed them names it.
func (Host) Listening() ([]core.Listener, error) {
	if _, err := exec.LookPath("lsof"); err == nil {
		output, _ := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN").Output()

		return fromLsof(string(output)), nil
	}

	if _, err := exec.LookPath("ss"); err == nil {
		output, _ := exec.Command("ss", "-ltn").Output()

		return fromSS(string(output)), nil
	}

	return nil, ErrNoPortTool
}

// fromLsof reads `lsof -nP -iTCP -sTCP:LISTEN`, whose first column is the command holding the
// port and whose address is the field before "(LISTEN)".
func fromLsof(output string) []core.Listener {
	listeners := []core.Listener{}

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "COMMAND" {
			continue
		}

		for _, field := range fields[1:] {
			match := listeningPort.FindStringSubmatch(field)
			if match == nil {
				continue
			}

			listeners = append(listeners, core.Listener{Port: match[1], Process: fields[0]})

			break
		}
	}

	return listeners
}

// fromSS reads `ss -ltn`, which names no process.
//
// The shell implementation took the first column of the matching line, which for `ss` is the word
// "LISTEN" — so a port conflict on Linux was reported as taken by "LISTEN". Reporting no name is
// what makes the message fall back to "processes on the host", which is true.
func fromSS(output string) []core.Listener {
	listeners := []core.Listener{}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[0] == "State" {
			continue
		}

		match := listeningPort.FindStringSubmatch(fields[3])
		if match == nil {
			continue
		}

		listeners = append(listeners, core.Listener{Port: match[1]})
	}

	return listeners
}

// InGroup answers whether the user belongs to a group.
func (Host) InGroup(name string) bool {
	current, err := user.Current()
	if err != nil {
		return false
	}

	ids, err := current.GroupIds()
	if err != nil {
		return false
	}

	for _, id := range ids {
		group, err := user.LookupGroupId(id)
		if err == nil && group.Name == name {
			return true
		}
	}

	return false
}

// Mkcert reports whether it is installed and where its local authority lives.
func (Host) Mkcert() (bool, string) {
	if _, err := exec.LookPath("mkcert"); err != nil {
		return false, ""
	}

	output, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return true, ""
	}

	return true, strings.TrimSpace(string(output))
}

// HostsEntry is whether /etc/hosts sends this domain anywhere.
func (Host) HostsEntry(domain string) bool {
	if domain == "" {
		return false
	}

	contents, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return false
	}

	pattern, err := regexp.Compile(`[ \t]` + regexp.QuoteMeta(domain) + `([ \t]|$)`)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(contents), "\n") {
		if pattern.MatchString(line) {
			return true
		}
	}

	return false
}

// ResolvesLocally is whether the name resolves to a loopback address.
//
// Resolving to something else is not enough: a domain that answers with a real internet address
// belongs to somebody, and pointing it at this machine in /etc/hosts is exactly what is wanted
// then.
func (Host) ResolvesLocally(domain string) bool {
	if domain == "" {
		return false
	}

	addresses, err := net.LookupHost(domain)
	if err != nil || len(addresses) == 0 {
		return false
	}

	for _, address := range addresses {
		parsed := net.ParseIP(address)
		if parsed == nil || !parsed.IsLoopback() {
			return false
		}
	}

	return true
}
