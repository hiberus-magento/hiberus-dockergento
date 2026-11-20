package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/contract"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// A command a project adds of its own: config/hm/commands/<name>.sh under the project's root.
//
// Ported from bin/run, which is what ran them until now: the same lookup, the same environment
// handed to the script, the same refusals in the same order, and the script's exit code as the
// tool's. bin/run answered only for names its router had no command for, and so does this — Run
// reaches it after every routed name, so a routed name always wins.
//
// The environment is the contract a project's scripts were written against, which is why it is
// built field by field the way bin/run exported it, down to the colour variables as the escaped
// strings the shell held rather than the characters they stand for.
//
// What bin/run checked in between and is not repeated here: validate_worktree refused only the
// names hm_alters_environment lists and noted only the names hm_operates_on_code lists, and every
// one of them is a command of the binary, so neither ever applied to a project command. The same
// is true of the commands validate_command exempted from the Docker checks.
//

// spawnProcess runs one program and returns its exit code: the docker checks, and the script.
// A test substitutes it.
var spawnProcess = func(program string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	command := exec.Command(program, args...)
	command.Env = env
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			// What the shell reports for a child killed by a signal
			return 128 + int(status.Signal()), nil
		}

		return exit.ExitCode(), nil
	}

	if err != nil {
		return 0, err
	}

	return 0, nil
}

// lookPath is `command -v`.
var lookPath = exec.LookPath

// projectCommand runs one of the project's commands, or refuses a name nobody answers for.
func projectCommand(name string, args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	pwd := here()
	resolved := resolveHMRoot(pwd)
	tool := toolRoot()

	run := &projectRun{
		name: name, pwd: pwd, tool: tool, jsonOutput: jsonOutput, stderr: stderr,
		env: inheritedEnvironment(),
	}

	script := filepath.Join(customCommandsDir(resolved.Root), name+".sh")
	if info, err := os.Stat(script); err != nil || !info.Mode().IsRegular() {
		descriptions, _ := readJSONObject(filepath.Join(tool, "data", "command_descriptions.json"))

		if _, declared := descriptions[name]; declared {
			return run.fail(exitUsage, "command_moved_to_binary",
				"'"+name+"' is part of the "+binaryName()+" binary now", binaryName()+" "+name)
		}

		return run.fail(exitUsage, "command_not_found", "Command not found: "+name, binaryName()+" --help")
	}

	if code := run.prepare(resolved); code != 0 {
		return code
	}

	if code := run.validate(); code != 0 {
		return code
	}

	return run.execute(script, args, stdout, stderr)
}

// projectRun is one invocation: what has been exported so far, and where its failures go.
type projectRun struct {
	name, pwd, tool string
	jsonOutput      bool
	stderr          io.Writer
	env             map[string]string
}

// inheritedEnvironment is this process's environment, without a registration an outer invocation
// may have handed down: it described another invocation's branch environment, and this one works
// out its own.
func inheritedEnvironment() map[string]string {
	env := map[string]string{}

	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if key == "HM_REGISTERED" || strings.HasPrefix(key, "HM_REGISTERED_") {
			continue
		}

		env[key] = value
	}

	return env
}

func (r *projectRun) set(key, value string) { r.env[key] = value }

func (r *projectRun) get(key string) string { return r.env[key] }

// prepare exports what bin/run exported before the script ran, in the order it did: the
// directories, the properties, the palette and the exit codes, the platform, the output decision,
// then the project's identity, its branch environment if it has one, and the compose command.
func (r *projectRun) prepare(resolved hmRoot) int {
	r.set("COMMAND_BIN_NAME", binaryName())
	r.set("COMMAND_TOOLNAME", "Hiberus Dockergento")
	r.set("COMMAND_BIN_DIR", r.tool)
	r.set("DATA_DIR", filepath.Join(r.tool, "data"))
	r.set("HELPERS_DIR", filepath.Join(r.tool, "console", "helpers"))
	r.set("TASKS_DIR", filepath.Join(r.tool, "console", "tasks"))
	r.set("COMPONENTS_DIR", filepath.Join(r.tool, "console", "components"))
	r.set("HM_ROOT", resolved.Root)
	r.set("HM_IS_WORKTREE", strconv.FormatBool(resolved.IsWorktree))
	r.set("HM_WORKTREE", resolved.Worktree)
	r.set("DOCKER_CONFIG_DIR", "config/docker")
	r.directories(resolved.Root)

	// The commands directory is the one the script was found in, which stays the main checkout's
	// even when a branch environment moves the root below
	r.set("COMMANDS_DIR", customCommandsDir(resolved.Root))

	r.loadProperties()
	r.palette()

	for name, code := range map[string]int{"OK": exitOK, "ERROR": exitError, "USAGE": exitUsage,
		"DOCKER": exitDocker, "PROJECT": exitProject, "SERVICE": exitService, "BLOCKED": exitBlocked} {
		r.set("HM_EXIT_"+name, strconv.Itoa(code))
	}

	r.set("MACHINE", helpMachine())
	if helpMachine() == "mac" {
		r.set("DOCKER_COMPOSE_FILE_MACHINE", r.get("DOCKER_COMPOSE_FILE_MAC"))
	} else {
		r.set("DOCKER_COMPOSE_FILE_MACHINE", r.get("DOCKER_COMPOSE_FILE_LINUX"))
	}

	format := "text"
	if r.jsonOutput {
		format = "json"
	}

	r.set("HM_OUTPUT_FORMAT", format)
	r.set("HM_NON_INTERACTIVE", os.Getenv("HM_NON_INTERACTIVE"))
	r.set("HM_FORCE", os.Getenv("HM_FORCE"))
	r.set("HM_NO_COLOR", os.Getenv("HM_NO_COLOR"))

	if r.get("HM_NON_INTERACTIVE") != "" {
		r.set("USE_DEFAULT_SETTINGS", "true")
	}

	// The palette again, now that --no-color is known: bin/run decided it twice for that reason
	r.palette()
	r.set("HM_COMMAND", r.name)

	if code := r.identity(); code != 0 {
		return code
	}

	r.composeCommand()
	r.set("HM_COMPOSE_VERSION_CACHE", r.get("HM_COMPOSE_VERSION_CACHE"))

	// set_environment_labels: the cheap labels every invocation exports
	r.set("HM_PROJECT", r.get("COMPOSE_PROJECT_NAME"))
	r.set("HM_PROFILE", orDefault(r.get("HM_PROFILE"), "full"))

	for _, key := range []string{"HM_AGENT", "HM_VERSION", "HM_MAGENTO"} {
		r.set(key, r.get(key))
	}

	return 0
}

func (r *projectRun) directories(root string) {
	r.set("CUSTOM_COMMANDS_DIR", customCommandsDir(root))
	r.set("CUSTOM_PROPERTIES_DIR", root+"/config/docker")
}

// loadProperties is load_properties: the tool's properties, the project's over them, every one
// exported.
func (r *projectRun) loadProperties() {
	for _, path := range []string{
		filepath.Join(r.tool, "data", "properties.json"),
		filepath.Join(r.get("CUSTOM_PROPERTIES_DIR"), "properties.json"),
	} {
		properties, err := readJSONObject(path)
		if err != nil {
			continue
		}

		for key, value := range properties {
			if text, ok := value.(string); ok {
				r.set(key, text)
			}
		}
	}
}

// palette is load_colors: the escape sequences as the shell held them, or every one empty.
func (r *projectRun) palette() {
	colours := [][2]string{
		{"BLUE", `\033[0;34m`}, {"GREEN", `\033[0;32m`}, {"CYAN", `\033[0;36m`}, {"RED", `\033[0;31m`},
		{"PURPLE", `\033[0;35m`}, {"BROWN", `\033[0;33m`}, {"WHITE", `\033[1;37m`},
		{"YELLOW", `\033[0;33m`}, {"BOLD", `\033[1m`}, {"COLOR_RESET", `\033[0m`},
	}

	on := coloured()

	for _, colour := range colours {
		if on {
			r.set(colour[0], colour[1])
		} else {
			r.set(colour[0], "")
		}
	}
}

// identity is the first half of docker_env_variables: the project's name, and the branch
// environment this is being run from when it has one.
func (r *projectRun) identity() int {
	name := r.get("COMPOSE_PROJECT_NAME")
	if name == "" {
		name = core.DeriveProjectName(r.get("HM_ROOT"))
	}

	if name == "" {
		return r.fail(exitProject, "no_project_name",
			"No project name can be derived from "+filepath.Base(r.get("HM_ROOT")),
			"Rename the directory, or set COMPOSE_PROJECT_NAME in config/docker/properties.json")
	}

	r.set("COMPOSE_PROJECT_NAME", name)
	r.set("HM_WORKTREE_ENVIRONMENT", "false")

	if r.get("HM_IS_WORKTREE") != "true" {
		return 0
	}

	project, err := newEngine(nil, nil, r.jsonOutput).Resolve(r.pwd)
	if err != nil || project.Worktree == nil {
		return 0
	}

	worktree := project.Worktree
	if worktree.Parent != name || worktree.Name != r.get("HM_WORKTREE") || worktree.Path == "" {
		return 0
	}

	home := os.Getenv("HM_WORKTREE_DIR")
	if home == "" {
		home = os.Getenv("HOME") + "/.hm/worktrees"
	}

	r.set("HM_WORKTREE_ENVIRONMENT", "true")
	r.set("HM_MAIN_ROOT", r.get("HM_ROOT"))
	r.set("HM_PARENT_PROJECT", name)
	r.set("HM_WORKTREE_OVERLAY", home+"/"+name+"/"+r.get("HM_WORKTREE")+".yml")
	r.set("HM_ROOT", worktree.Path)
	r.directories(worktree.Path)
	r.loadProperties()

	// After the properties, which name the main environment's project again
	r.set("COMPOSE_PROJECT_NAME", worktree.Project)
	r.set("HM_PROFILE", worktree.Profile)
	r.set("DOMAIN", worktree.Domain)

	if worktree.Profile == "agent" {
		r.set("HM_AGENT", "true")
	}

	return 0
}

// composeCommand is DOCKER_COMPOSE as bin/run built it: the files relative to the directory the
// command was run in for a main checkout, absolute with --project-directory from a worktree.
func (r *projectRun) composeCommand() {
	compose := r.detectCompose()
	root := r.get("HM_ROOT")
	file := r.get("DOCKER_COMPOSE_FILE")
	machine := r.get("DOCKER_COMPOSE_FILE_MACHINE")

	var command string

	switch {
	case r.get("HM_WORKTREE_ENVIRONMENT") == "true":
		command = compose + " --project-directory " + root + " -f " + root + "/" + file + " -f " + root + "/" + machine +
			" -f " + r.get("HM_WORKTREE_OVERLAY")
	case r.get("HM_IS_WORKTREE") == "true":
		command = compose + " --project-directory " + root + " -f " + root + "/" + file + " -f " + root + "/" + machine
	default:
		command = compose + " -f " + file + " -f " + machine
	}

	proxy := strings.TrimSuffix(file, ".yml") + ".proxy.yml"
	if r.get("HM_WORKTREE_ENVIRONMENT") != "true" && r.isFile(proxy) {
		command += " -f " + proxy
	}

	r.set("DOCKER_COMPOSE", command)
}

// detectCompose is get_docker_compose_cmd, with its cache: the answer only changes when Docker is
// reinstalled, so it is kept against the modification time of the docker binary.
func (r *projectRun) detectCompose() string {
	docker, _ := lookPath("docker")
	token := ""

	if docker != "" {
		token = fileMtime(docker)
		if cached, ok := r.cacheRead("compose-cmd", token); ok {
			return cached
		}

		if code, err := spawnProcess(docker, []string{"compose", "version"}, r.environment(), nil, io.Discard, io.Discard); err == nil && code == 0 {
			r.cacheWrite("compose-cmd", token, "docker compose")

			return "docker compose"
		}
	}

	if _, err := lookPath("docker-compose"); err == nil {
		if docker != "" {
			r.cacheWrite("compose-cmd", token, "docker-compose")
		}
	}

	return "docker-compose"
}

// validate is validate_command for a name it did not exempt: the daemon, then the project's
// compose configuration, which is skipped while its files have not changed since it last passed.
func (r *projectRun) validate() int {
	if code, err := spawnProcess("docker", []string{"info"}, r.environment(), nil, io.Discard, io.Discard); err != nil || code != 0 {
		return r.fail(exitDocker, "docker_unavailable", "Docker is not running", "Start Docker and try again")
	}

	key := "validated" + strings.ReplaceAll(r.get("HM_ROOT"), "/", "-")
	token := ""

	for _, file := range []string{r.get("DOCKER_COMPOSE_FILE"), r.get("DOCKER_COMPOSE_FILE_MACHINE")} {
		if r.isFile(file) {
			token += ":" + fileToken(r.at(file))
		}
	}

	if token != "" {
		if _, ok := r.cacheRead(key, token); ok {
			return 0
		}
	}

	words := strings.Fields(r.get("DOCKER_COMPOSE"))

	var problem bytes.Buffer

	valid := false
	if len(words) > 0 {
		code, err := spawnProcess(words[0], append(words[1:], "config", "-q"), r.environment(), nil, io.Discard, &problem)
		valid = err == nil && code == 0
	}

	if valid {
		if token != "" {
			r.cacheWrite(key, token, "valid")
		}

		return 0
	}

	message := "This directory is not a configured Hiberus Dockergento project, or its Docker configuration is invalid"
	if detail := strings.TrimRight(problem.String(), "\n"); detail != "" {
		message += ": " + detail
	}

	return r.fail(exitProject, "project_not_configured", message, binaryName()+" setup")
}

// execute runs the script with the command's own arguments, attached to this terminal, and
// answers with its exit code.
//
// An interruption belongs to the script: it reaches it from the terminal directly, and this
// process waits for it to finish rather than dying first and leaving it behind.
func (r *projectRun) execute(script string, args []string, stdout, stderr io.Writer) int {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGQUIT)

	defer signal.Stop(interrupts)

	code, err := spawnProcess(script, args, r.environment(), os.Stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", script, err)

		if errors.Is(err, os.ErrNotExist) {
			return 127
		}

		return 126
	}

	return code
}

// environment is what the script and the checks are handed.
func (r *projectRun) environment() []string {
	entries := make([]string, 0, len(r.env))
	for key, value := range r.env {
		entries = append(entries, key+"="+value)
	}

	sort.Strings(entries)

	return entries
}

// fail is hm_fail: the envelope on stderr under JSON, the painted message and hint otherwise, and
// the exit code that says which kind of failure it was.
func (r *projectRun) fail(code int, kind, message, hint string) int {
	if r.jsonOutput {
		body := struct {
			SchemaVersion int    `json:"schema_version"`
			Command       string `json:"command"`
			OK            bool   `json:"ok"`
			Error         struct {
				Code    int    `json:"code"`
				Type    string `json:"type"`
				Message string `json:"message"`
				Hint    string `json:"hint"`
			} `json:"error"`
		}{SchemaVersion: contract.SchemaVersion, Command: orDefault(r.name, binaryName())}

		body.Error.Code, body.Error.Type, body.Error.Message, body.Error.Hint = code, kind, message, hint

		encoder := json.NewEncoder(r.stderr)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		encoder.Encode(body) //nolint:errcheck

		return code
	}

	page := &helpPage{colour: coloured()}
	page.decorated("\033[0;31m", "\n"+message+"\n")

	if hint != "" {
		page.plain("\n  ")
		page.codeText(hint)
		page.plain("\n\n")
	}

	io.WriteString(r.stderr, page.out.String()) //nolint:errcheck

	return code
}

// at is a path as the shell would have opened it: relative to the directory the command was run
// in.
func (r *projectRun) at(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(r.pwd, path)
}

func (r *projectRun) isFile(path string) bool {
	info, err := os.Stat(r.at(path))

	return err == nil && info.Mode().IsRegular()
}

//
// The on-disk cache of console/helpers/cache.sh, shared with it: the same directory, the same
// keys, a token on the first line and the value on the second.
//

func (r *projectRun) cacheDir() string {
	if dir := os.Getenv("HM_CACHE_DIR"); dir != "" {
		return dir
	}

	return os.Getenv("HOME") + "/.hm/cache"
}

func (r *projectRun) cacheRead(key, token string) (string, bool) {
	contents, err := os.ReadFile(filepath.Join(r.cacheDir(), key))
	if err != nil {
		return "", false
	}

	lines := strings.SplitN(string(contents), "\n", 3)
	if len(lines) < 2 || lines[0] != token || lines[1] == "" {
		return "", false
	}

	return lines[1], true
}

func (r *projectRun) cacheWrite(key, token, value string) {
	if err := os.MkdirAll(r.cacheDir(), 0o755); err != nil {
		return
	}

	os.WriteFile(filepath.Join(r.cacheDir(), key), []byte(token+"\n"+value+"\n"), 0o644) //nolint:errcheck,gosec
}

// fileMtime and fileToken are stat's: of the link itself, as stat(1) reports it without -L.
func fileMtime(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return "0"
	}

	return strconv.FormatInt(info.ModTime().Unix(), 10)
}

func fileToken(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return "0"
	}

	return strconv.FormatInt(info.ModTime().Unix(), 10) + "-" + strconv.FormatInt(info.Size(), 10)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
