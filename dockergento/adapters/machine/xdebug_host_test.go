package machine

import "testing"

// These three parsers take a string and return a value. The platform never enters them, so they
// are provable on any CI runner, exactly like fromLsof/fromSS above — no OS-conditional skip and
// no dependency on the live operating system.

const procVersionWSL1 = `Linux version 4.4.0-43-Microsoft (Microsoft@Microsoft.com) (gcc version 5.4.0 (GCC) ) #1-Microsoft Wed Dec 31 14:42:53 PST 2014`

const procVersionWSL2 = `Linux version 5.15.90.1-microsoft-standard-WSL2 (oe-user@oe-host) (x86_64-msft-linux-gcc (GCC) 9.3.0, GNU ld (GNU Binutils) 2.34.0.20200220) #1 SMP Fri Jan 27 02:56:13 UTC 2023`

const procVersionOrdinary = `Linux version 5.15.0-91-generic (buildd@lcy02-amd64-078) (gcc (Ubuntu 11.4.0-1ubuntu1~22.04) 11.4.0, GNU ld (GNU Binutils for Ubuntu) 2.38) #101-Ubuntu SMP Tue Nov 14 13:30:08 UTC 2023`

// procVersionWSL2WithMicrosoft is the adversarial fixture: a WSL2-shaped kernel string that also
// carries a capital-M "Microsoft" token, here in a build-host field. If the parser kept the
// shell's original WSL1-first check order, this would be misdetected as WSL1.
const procVersionWSL2WithMicrosoft = `Linux version 5.15.90.1-microsoft-standard-WSL2 (Microsoft@example.com) (x86_64-msft-linux-gcc (GCC) 9.3.0, GNU ld (GNU Binutils) 2.34.0.20200220) #1 SMP Fri Jan 27 02:56:13 UTC 2023`

const ipAddrDocker0Present = `3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default
    link/ether 02:42:0a:5b:2e:1a brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:aff:fe5b:2e1a/64 scope link
       valid_lft forever preferred_lft forever
`

// ipAddrDocker0OnlyInet6 has no plain "inet " line at all, only "inet6" — simulating a docker0
// that only shows a link-local IPv6 line, or reordered output. The parser must not fall back to
// matching inet6 when no inet line exists.
const ipAddrDocker0OnlyInet6 = `3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN group default
    link/ether 02:42:0a:5b:2e:1a brd ff:ff:ff:ff:ff:ff
    inet6 fe80::42:aff:fe5b:2e1a/64 scope link
       valid_lft forever preferred_lft forever
`

const ifconfigDocker0Present = `docker0   Link encap:Ethernet  HWaddr 02:42:0a:5b:2e:1a
          inet addr:172.17.0.1  Bcast:172.17.255.255  Mask:255.255.0.0
          inet6 addr: fe80::42:aff:fe5b:2e1a/64 Scope:Link
          UP BROADCAST MULTICAST  MTU:1500  Metric:1
`

func TestHostFromProcVersionWSL1(t *testing.T) {
	host, ok := hostFromProcVersion(procVersionWSL1)

	if !ok || host != "10.0.75.1" {
		t.Fatalf("hostFromProcVersion(WSL1) = (%q, %v), want (\"10.0.75.1\", true)", host, ok)
	}
}

func TestHostFromProcVersionWSL2(t *testing.T) {
	host, ok := hostFromProcVersion(procVersionWSL2)

	if !ok || host != "host.docker.internal" {
		t.Fatalf("hostFromProcVersion(WSL2) = (%q, %v), want (\"host.docker.internal\", true)", host, ok)
	}
}

func TestHostFromProcVersionOrdinaryLinux(t *testing.T) {
	host, ok := hostFromProcVersion(procVersionOrdinary)

	if ok || host != "" {
		t.Fatalf("hostFromProcVersion(ordinary Linux) = (%q, %v), want (\"\", false)", host, ok)
	}
}

// TestHostFromProcVersionWSL2WinsOverMicrosoft is the adversarial case Decision 4 exists for: a
// WSL2 string that also contains a plain "Microsoft" token must still resolve to
// host.docker.internal, proving microsoft-standard is checked ahead of plain Microsoft.
func TestHostFromProcVersionWSL2WinsOverMicrosoft(t *testing.T) {
	host, ok := hostFromProcVersion(procVersionWSL2WithMicrosoft)

	if !ok || host != "host.docker.internal" {
		t.Fatalf("hostFromProcVersion(WSL2 with Microsoft token) = (%q, %v), want (\"host.docker.internal\", true)", host, ok)
	}
}

func TestHostFromProcVersionEmpty(t *testing.T) {
	host, ok := hostFromProcVersion("")

	if ok || host != "" {
		t.Fatalf("hostFromProcVersion(\"\") = (%q, %v), want (\"\", false)", host, ok)
	}
}

func TestHostFromIPAddrPresent(t *testing.T) {
	host, ok := hostFromIPAddr(ipAddrDocker0Present)

	if !ok || host != "172.17.0.1" {
		t.Fatalf("hostFromIPAddr(present) = (%q, %v), want (\"172.17.0.1\", true)", host, ok)
	}
}

// TestHostFromIPAddrDoesNotMatchInet6 is the inet6 boundary success criterion: a fixture whose
// only "inet"-prefixed line is "inet6" must not be mistaken for a resolved address.
func TestHostFromIPAddrDoesNotMatchInet6(t *testing.T) {
	host, ok := hostFromIPAddr(ipAddrDocker0OnlyInet6)

	if ok || host != "" {
		t.Fatalf("hostFromIPAddr(only inet6) = (%q, %v), want (\"\", false)", host, ok)
	}
}

// TestHostFromIPAddrEmpty covers the absent interface too, and deliberately as one test rather
// than two: when docker0 does not exist, `ip` writes its complaint to stderr and leaves stdout
// empty, and stdout is all this function reads. So "the interface is absent" and "the output is
// empty" are the same input here, not two scenarios. Two tests under two names would read as
// twice the coverage they are.
func TestHostFromIPAddrEmpty(t *testing.T) {
	host, ok := hostFromIPAddr("")

	if ok || host != "" {
		t.Fatalf("hostFromIPAddr(\"\") = (%q, %v), want (\"\", false)", host, ok)
	}
}

func TestHostFromIfconfigPresent(t *testing.T) {
	host, ok := hostFromIfconfig(ifconfigDocker0Present)

	if !ok || host != "172.17.0.1" {
		t.Fatalf("hostFromIfconfig(present) = (%q, %v), want (\"172.17.0.1\", true)", host, ok)
	}
}

func TestHostFromIfconfigEmpty(t *testing.T) {
	host, ok := hostFromIfconfig("")

	if ok || host != "" {
		t.Fatalf("hostFromIfconfig(\"\") = (%q, %v), want (\"\", false)", host, ok)
	}
}
