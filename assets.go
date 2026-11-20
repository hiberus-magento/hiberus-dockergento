// Package hm is the files the hm binary ships with: the data it reads, the compose templates it
// renders and the skills it installs into projects.
//
// They are compiled into the binary, so an installation is one file and never a checkout. The
// package sits at the root of the module because go:embed cannot reach a parent directory, and
// these directories are where contributors edit them.
package hm

import (
	"embed"
	"io/fs"
)

// The data files are listed one by one rather than as data/*.json: data/config.json is a
// contributor's own install settings, ignored by git, and a glob would compile it into the binary.
//
//go:embed data/ai-exclusions.json
//go:embed data/ai-platforms.json
//go:embed data/ai-repositories.json
//go:embed data/ai-skill-types.json
//go:embed data/command_descriptions.json
//go:embed data/default_files_magento.json
//go:embed data/equivalent_versions.json
//go:embed data/local_settings.json
//go:embed data/properties.json
//go:embed data/requirements.json
//go:embed docker-compose/*.yml
//go:embed skills
var embedded embed.FS

// Embedded is the copy compiled into the binary.
func Embedded() fs.FS { return embedded }

// Assets is what the tool reads: the embedded copy. Nothing is ever looked for next to the
// executable.
func Assets() fs.FS { return embedded }
