package machine

import (
	"net"
	"strings"
)

// Resolver asks this machine how a name resolves.
type Resolver struct{}

// ResolvesLocally reports whether a name reaches this machine.
//
// Resolving to something else is not enough: a domain that answers with a real internet address
// belongs to somebody, and pointing it at this machine in the hosts file is exactly what is wanted
// then. So every answer has to be a loopback address, and a name that resolves to nothing is not
// resolving here.
func (Resolver) ResolvesLocally(domain string) bool {
	if domain == "" {
		return false
	}

	addresses, err := net.LookupHost(domain)
	if err != nil || len(addresses) == 0 {
		return false
	}

	for _, address := range addresses {
		if !loopback(address) {
			return false
		}
	}

	return true
}

func loopback(address string) bool {
	if strings.HasPrefix(address, "127.") {
		return true
	}

	parsed := net.ParseIP(address)

	return parsed != nil && parsed.IsLoopback()
}
