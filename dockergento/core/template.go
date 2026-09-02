package core

import (
	"fmt"
	"strings"
)

//
// A frozen copy of a database's data directory.
//
// `db snapshot` writes a dump: portable, small, and readable a year and two server versions later.
// This is the other half — a byte copy of the data directory in a Docker volume, which is not
// portable at all and is available again in seconds instead of the tens of minutes an import of
// the same data costs. One is for keeping, the other for standing environments up.
//

// The label every template volume carries, which is how they are found without a registry.
const TemplateLabel = "hm.template"

// Template is one of them.
type Template struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Size    string `json:"size"`
	// Bytes is what Size was made from, and it is not reported in a listing: what a person reads
	// there is the size, and the number is for whoever asked for the freeze.
	Bytes   int64  `json:"-"`
	Image   string `json:"db_image"`
	Created string `json:"created"`
	Volume  string `json:"volume"`
}

// TemplateAddress is how a template is named to a person: the project it came from and its own
// name, so that one project can build an environment from another's.
func TemplateAddress(project, name string) string { return project + "/" + name }

// TemplateVolume is the volume that holds it.
func TemplateVolume(project, name string) string {
	return fmt.Sprintf("hm-template-%s-%s", project, name)
}

// ParseTemplate reads `<name>` or `<project>/<name>`, defaulting to this project.
func ParseTemplate(address, project string) (string, string) {
	if at := strings.Index(address, "/"); at >= 0 {
		return address[:at], address[at+1:]
	}

	return project, address
}

// ValidTemplateName reports whether a name can be used. It ends up in a volume name and in a
// path, so what it may contain is not a matter of taste.
func ValidTemplateName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return false
	}

	for _, character := range name {
		switch {
		case character >= 'A' && character <= 'Z',
			character >= 'a' && character <= 'z',
			character >= '0' && character <= '9',
			character == '.', character == '_', character == '-':
		default:
			return false
		}
	}

	return true
}

// HumanSize is what a person reads instead of a number of bytes.
func HumanSize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}

	value := float64(bytes)
	at := 0

	for value >= 1024 && at < len(units)-1 {
		value /= 1024
		at++
	}

	if at == 0 {
		return fmt.Sprintf("%d%s", int64(value), units[at])
	}

	return fmt.Sprintf("%.1f%s", value, units[at])
}
