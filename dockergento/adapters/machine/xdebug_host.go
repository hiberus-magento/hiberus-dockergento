package machine

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// ErrNoBridgeAddress is returned on Linux when docker0 exists but no tool on this machine could
// resolve its address. A check that cannot look must not report that everything is fine: xdebug
// gets enabled with no address to call back on, and a breakpoint that never fires gives no clue
// why.
var ErrNoBridgeAddress = errNoBridgeAddress{}

type errNoBridgeAddress struct{}

func (errNoBridgeAddress) Error() string { return "no address found for the docker0 bridge" }

// XdebugHost is the address xdebug inside a container should call back to reach the developer's
// IDE on the host.
//
// Not applicable outside Linux: `("", nil)` on macOS, nothing to patch. On Linux, WSL1 and WSL2
// each have a fixed address; anything else is read from docker0 via `ip`, falling back to
// `ifconfig`. A docker0 that exists but cannot be resolved is `("", ErrNoBridgeAddress)` — distinct
// from "not applicable," so a caller can tell a normal Mac from a broken Linux box.
func (Host) XdebugHost() (host string, err error) {
	if runtime.GOOS != "linux" {
		return "", nil
	}

	if content, readErr := os.ReadFile("/proc/version"); readErr == nil {
		if resolved, ok := hostFromProcVersion(string(content)); ok {
			return resolved, nil
		}
	}

	if _, lookErr := exec.LookPath("ip"); lookErr == nil {
		output, _ := exec.Command("ip", "addr", "show", "docker0").Output()
		if resolved, ok := hostFromIPAddr(string(output)); ok {
			return resolved, nil
		}
	} else if _, lookErr := exec.LookPath("ifconfig"); lookErr == nil {
		output, _ := exec.Command("ifconfig", "docker0").Output()
		if resolved, ok := hostFromIfconfig(string(output)); ok {
			return resolved, nil
		}
	}

	return "", ErrNoBridgeAddress
}

// hostFromProcVersion reads /proc/version and answers WSL1's and WSL2's fixed xdebug host, or
// nothing on ordinary Linux.
//
// microsoft-standard is checked before plain Microsoft, both as case-sensitive substring checks.
// This is deliberate, not the shell's original order: a WSL2 kernel string can carry a capital-M
// "Microsoft" token elsewhere (a build-host field, for instance), and checking the more specific
// token first is what keeps that string from being misdetected as WSL1.
func hostFromProcVersion(content string) (host string, ok bool) {
	if strings.Contains(content, "microsoft-standard") {
		return "host.docker.internal", true
	}

	if strings.Contains(content, "Microsoft") {
		return "10.0.75.1", true
	}

	return "", false
}

var ipAddrInetLine = regexp.MustCompile(`^\s*inet\s+(\S+)`)

// hostFromIPAddr reads `ip addr show docker0`'s output and answers its IPv4 address.
//
// Matches a line beginning (after leading whitespace) with "inet" followed by a space, the way
// the shell's `grep "inet\b"` does — deliberately excluding "inet6", which shares the "inet"
// prefix but names a different address family entirely.
func hostFromIPAddr(output string) (host string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		match := ipAddrInetLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		address := strings.SplitN(match[1], "/", 2)[0]
		if address == "" {
			continue
		}

		return address, true
	}

	return "", false
}

// hostFromIfconfig reads `ifconfig docker0`'s output (net-tools legacy format) and answers its
// IPv4 address.
//
// Replicates the shell's two-step split — `cut -d ':' -f 2` then `cut -d ' ' -f 1` — rather than
// a single regex: net-tools versions order "Bcast"/"Mask" differently, and a pattern tuned to one
// sample would silently diverge on another.
func hostFromIfconfig(output string) (host string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "inet addr") {
			continue
		}

		colonFields := strings.SplitN(trimmed, ":", 3)
		if len(colonFields) < 2 {
			continue
		}

		spaceFields := strings.Fields(colonFields[1])
		if len(spaceFields) == 0 {
			continue
		}

		return spaceFields[0], true
	}

	return "", false
}
