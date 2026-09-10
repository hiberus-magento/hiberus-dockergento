package core

import (
	"strings"
)

//
// The machine's own name resolution: /etc/hosts.
//
// Every entry costs a password prompt, and they were never removed — twenty-three of them on the
// machine this was written on, several from projects that no longer exist. Two things follow from
// that. What this tool adds carries a marker, so it can find its own and leave alone anything a
// person wrote. And nothing is added when the name already resolves here, because then the entry
// buys nothing and stays forever.
//

// HostsMarker is what says who wrote a line. It is the whole point of it: an entry with nothing to
// say where it came from accumulates for as long as the machine lives and nobody dares delete one.
func HostsMarker(binary string) string { return "# added by " + binary }

// HostsEntries are the lines this tool writes for a domain: one per address family.
//
// One line, `0.0.0.0 ::1 <domain>`, is what this wrote until 1.5.0, and it is malformed. A hosts
// line carries one address and everything after it is an alias, so `::1` was read as an alias and
// the domain resolved to 0.0.0.0 only. Chrome and curl reach that address anyway, because the
// kernel sends it to the loopback interface, so the only thing that ever complained was Firefox,
// which refuses it — and what it reports is a CORS error with no status.
//
// Both lines carry the marker, because the removal path finds its own by it.
func HostsEntries(domain, binary string) string {
	marker := HostsMarker(binary)

	return "127.0.0.1 " + domain + " " + marker + "\n::1 " + domain + " " + marker
}

// HostsHasLegacy reports whether a hosts file still has the malformed entry for a domain.
//
// It is a question of its own, and it is asked before whether the domain resolves, because it
// does resolve with that line in the file: to 0.0.0.0. Asking about the result would leave it
// there for ever.
func HostsHasLegacy(contents, domain string) bool {
	for _, line := range strings.Split(contents, "\n") {
		if isLegacy(line, domain) {
			return true
		}
	}

	return false
}

// isLegacy reports whether a line is the entry this tool used to write for a domain.
//
// Exactly that shape: a hand-written line pointing a name at 0.0.0.0 is somebody's decision, and
// this only heals what this tool wrote.
func isLegacy(line, domain string) bool {
	parts := fields(line)

	if len(parts) < 3 || parts[0] != "0.0.0.0" || parts[1] != "::1" {
		return false
	}

	for _, part := range parts[2:] {
		if part == domain {
			return true
		}
	}

	return false
}

// withoutLegacy is the file with the malformed entries for a domain taken out, and every other
// line — including malformed ones naming other domains — left as it was.
func withoutLegacy(contents, domain string) string {
	kept := []string{}

	for _, line := range strings.Split(contents, "\n") {
		if isLegacy(line, domain) {
			continue
		}

		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}

// HostsHas reports whether a hosts file already resolves a domain, however the line got there.
//
// Whoever wrote it, the name resolves, and adding a second line for it would be one more line
// nobody can attribute.
func HostsHas(contents, domain string) bool {
	for _, line := range strings.Split(contents, "\n") {
		if names(line)[domain] {
			return true
		}
	}

	return false
}

// HostsAdded reports whether *this tool* added an entry for a domain, which is a different
// question: it is the one that decides whether removing it is this tool's business.
func HostsAdded(contents, domain, binary string) bool {
	marker := HostsMarker(binary)

	for _, line := range strings.Split(contents, "\n") {
		if strings.Contains(line, marker) && names(line)[domain] {
			return true
		}
	}

	return false
}

// HostsMarked are the domains this tool's own entries name, each one once and in the order the
// file has them.
//
// Once, because there is a line per address family and what a caller asks about is the domain: an
// offer to clean an entry up would otherwise be made twice for the same name.
func HostsMarked(contents, marker string) []string {
	domains := []string{}
	seen := map[string]bool{}

	for _, line := range strings.Split(contents, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}

		for _, field := range fields(line) {
			if isAddress(field) || strings.Contains(field, ":") || !strings.Contains(field, ".") {
				continue
			}

			if !seen[field] {
				seen[field] = true
				domains = append(domains, field)
			}

			break
		}
	}

	return domains
}

// WithoutHost is the file with this tool's entries for a domain taken out, and everything else
// left exactly as it was — including the lines somebody wrote by hand, which are not ours to
// delete.
func WithoutHost(contents, domain, binary string) string {
	marker := HostsMarker(binary)
	kept := []string{}

	for _, line := range strings.Split(contents, "\n") {
		if strings.Contains(line, marker) && names(line)[domain] {
			continue
		}

		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}

// WithHost is the file with an entry for a domain added at the end, when there is not one already,
// and with the malformed entry this tool used to write taken out first.
//
// That order is the whole of it: the malformed line resolves the domain, so anything that asked
// whether the domain resolves would keep it.
func WithHost(contents, domain, binary string) string {
	contents = withoutLegacy(contents, domain)

	if HostsHas(contents, domain) {
		return contents
	}

	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}

	return contents + HostsEntries(domain, binary) + "\n"
}

// names is the set of names a hosts line resolves, which is everything on it but the addresses and
// whatever a comment says.
//
// A line is `<addresses...> <names...>` and anything from a `#` is a comment — except that this
// tool's marker is a comment, so what is compared is only what comes before it.
func names(line string) map[string]bool {
	found := map[string]bool{}

	for _, field := range fields(line) {
		// The addresses at the start are not names. Told apart by what they are made of rather
		// than by position: a line may begin with one address or with several
		if strings.ContainsAny(field, ":") || isAddress(field) {
			continue
		}

		found[field] = true
	}

	return found
}

// fields are what a hosts line says, with whatever a comment says taken off — except that this
// tool's marker is a comment, so what is read is only what comes before it.
func fields(line string) []string {
	if at := strings.Index(line, "#"); at >= 0 {
		line = line[:at]
	}

	return strings.Fields(line)
}

// isAddress reports whether a field is an IPv4 address, which is digits and dots and nothing else.
func isAddress(field string) bool {
	if field == "" {
		return false
	}

	for _, character := range field {
		if (character < '0' || character > '9') && character != '.' {
			return false
		}
	}

	return true
}
