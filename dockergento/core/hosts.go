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

// HostsEntry is the line this tool writes for a domain.
func HostsEntry(domain, binary string) string {
	return "0.0.0.0 ::1 " + domain + " " + HostsMarker(binary)
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

// WithHost is the file with an entry for a domain added at the end, when there is not one already.
func WithHost(contents, domain, binary string) string {
	if HostsHas(contents, domain) {
		return contents
	}

	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}

	return contents + HostsEntry(domain, binary) + "\n"
}

// names is the set of names a hosts line resolves, which is everything on it but the addresses and
// whatever a comment says.
//
// A line is `<addresses...> <names...>` and anything from a `#` is a comment — except that this
// tool's marker is a comment, so what is compared is only what comes before it.
func names(line string) map[string]bool {
	if at := strings.Index(line, "#"); at >= 0 {
		line = line[:at]
	}

	found := map[string]bool{}

	for _, field := range strings.Fields(line) {
		// The addresses at the start are not names. Told apart by what they are made of rather
		// than by position: a line may begin with one address or with several
		if strings.ContainsAny(field, ":") || isAddress(field) {
			continue
		}

		found[field] = true
	}

	return found
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
