package core

import "strings"

//
// What a branch is called once it is an environment.
//
// A branch name is not a host name, a volume name or a file name. This makes it all three, and it
// is deliberately lossy: `feature/x` and `Feature_X` both become `feature-x`, and the command
// refuses the second rather than inventing a name nobody chose — here the name decides which
// containers, which volumes and which database are used.
//

// Slug turns a branch name into something usable as all three.
func Slug(branch string) string {
	lowered := strings.ToLower(branch)

	var slug strings.Builder

	for _, character := range lowered {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			slug.WriteRune(character)
		default:
			slug.WriteByte('-')
		}
	}

	// No leading, trailing or repeated dashes: they are legal in a host name only in the middle
	result := slug.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	return strings.Trim(result, "-")
}

// The profiles, and what each keeps running.
//
// A branch environment that also runs Varnish, TLS termination, a mail catcher and a message queue
// costs more than the branch is worth; one without a search engine fails on the first reindex,
// which is not a surprise to leave in an environment meant for unattended work.
var profiles = map[string][]string{
	"lite":  {"phpfpm"},
	"agent": {"phpfpm", "nginx", "db", "search", "redis"},
	"full":  nil, // everything the project has
}

// ProfileKeeps is the services a profile runs, and whether the profile exists at all. An empty
// list with ok true means everything.
func ProfileKeeps(profile string) ([]string, bool) {
	keeps, ok := profiles[profile]

	return keeps, ok
}

// Profiles is what may be asked for.
func Profiles() string { return "lite agent full" }

// WebService is the service a branch environment is reached through, and the port behind it.
func WebService(profile string) (string, string) {
	if profile == "full" {
		return "varnish", "6081"
	}

	return "nginx", "8080"
}
