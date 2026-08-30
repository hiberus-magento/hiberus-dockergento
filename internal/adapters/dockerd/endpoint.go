package dockerd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Endpoint works out where the daemon is, the way the docker CLI does.
//
// This is not a detail: the SDK's own environment lookup reads DOCKER_HOST and nothing else,
// while every installation the department uses — Colima, Docker Desktop, Rancher — keeps the
// socket in the CLI's *context store* and leaves DOCKER_HOST unset. A binary that only knew
// about the variable would report "Docker is not running" on a machine where Docker is running,
// which is the worst kind of wrong answer.
//
// The shell implementation got this for free by invoking `docker`. The Go one has to do it, and
// this is what that costs.
func Endpoint() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}

	if host := contextEndpoint(configDir(), currentContext()); host != "" {
		return host
	}

	return ""
}

func configDir() string {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".docker")
}

// currentContext is what the environment says, then what the configuration says. The variable
// wins because that is what `docker --context` sets for a single command.
func currentContext() string {
	if name := os.Getenv("DOCKER_CONTEXT"); name != "" {
		return name
	}

	contents, err := os.ReadFile(filepath.Join(configDir(), "config.json"))
	if err != nil {
		return ""
	}

	var configuration struct {
		CurrentContext string `json:"currentContext"`
	}

	if err := json.Unmarshal(contents, &configuration); err != nil {
		return ""
	}

	return configuration.CurrentContext
}

// contextEndpoint reads the docker endpoint of a named context from the store.
//
// The store keys directories by the SHA-256 of the context name, which is undocumented in the
// sense that nobody promised it, and stable in the sense that every docker CLI depends on it.
func contextEndpoint(dir, name string) string {
	if dir == "" || name == "" || name == "default" {
		return ""
	}

	digest := sha256.Sum256([]byte(name))
	file := filepath.Join(dir, "contexts", "meta", hex.EncodeToString(digest[:]), "meta.json")

	contents, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	var metadata struct {
		Endpoints struct {
			Docker struct {
				Host string `json:"Host"`
			} `json:"docker"`
		} `json:"Endpoints"`
	}

	if err := json.Unmarshal(contents, &metadata); err != nil {
		return ""
	}

	return metadata.Endpoints.Docker.Host
}
