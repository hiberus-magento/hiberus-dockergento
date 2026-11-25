package app

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// ai-init, ai-pull and ai-reset: what a project asked for in config/docker/ai-properties.json,
// the skills and agents installed from it into each platform's directories, and what the tool
// recorded about each one in config/docker/ai-registration.json, so that only what it installed
// is ever updated or removed.
//
// Ported from console/commands/ai-{init,pull,reset}.sh and the tasks they sourced
// (console/tasks/ai_{registration,download,extract,wizard}.sh). Every message is the script's,
// on the stream the script put it on: that is why the console below has as many ways to print as
// the script had, and why a message that ended without a newline there ends without one here.
//

// The ways the scripts printed a message.
const (
	// AIDecorated is print_info and its siblings: stdout and painted in text, stderr and plain
	// under --json.
	AIDecorated = iota
	// AIDecoratedErr is the same with `>&2` after it: stderr either way, painted only in text.
	AIDecoratedErr
	// AILine is print_info_line and its siblings: stderr, painted, and a newline after it.
	AILine
)

// The levels a message is printed at, which decide its colour.
const (
	AIInfo    = "info"
	AIWarning = "warning"
	AIError   = "error"
	AIHeader  = "header"
)

// AIConsole is the terminal, as far as these commands see it.
type AIConsole interface {
	// Say prints a message at a level, the way one of the script's printers did.
	Say(level, text string, how int)
	// Stdout and Stderr write as they are, as echo and printf did.
	Stdout(text string)
	Stderr(text string)
	// ReadLine is `read -r`: one line, white space around it taken off, "" at the end of input.
	ReadLine() string
	// Fail is hm_fail without the exit: the error in the active format, on stderr.
	Fail(code int, kind, message, hint string)
	// JSON is whether the output is JSON.
	JSON() bool
}

// AITools is the three commands against one project.
type AITools struct {
	// Dir is where the command runs. The scripts used paths relative to $PWD for everything — the
	// two files, the platforms' directories and every path the registration records — so every
	// relative path is resolved here and printed as it is.
	Dir string

	// Tool is the files the tool ships with: its data/ and the skills/ that come with it.
	Tool fs.FS

	// Repositories is the path of the list of repositories ai-pull installs from
	// (HM_AI_REPOSITORIES). Empty is the tool's own data/ai-repositories.json, which ai-init
	// always reads.
	Repositories string

	// Binary is the name the tool was invoked as, for the hint of a question nobody can answer.
	Binary string

	// Version is what an installation from the tool's own skills records as its version.
	Version func() string

	// Exec runs a program: curl, tar or git. quiet throws its error stream away, as `2>/dev/null`
	// did.
	Exec func(quiet bool, name string, args ...string) error

	// HasGit is `command -v git`.
	HasGit func() bool

	// Sleep is the wait between two attempts at a download.
	Sleep func(time.Duration)

	// Now is the time recorded against what is configured and installed.
	Now func() time.Time

	// TempRoot is where downloads are unpacked: /tmp, as the script's mktemp template said.
	TempRoot string

	Console AIConsole

	// Where what is being installed came from, recorded against each resource
	origin, url, branch, version string
}

// AIInitOptions is what ai-init was given on the command line.
type AIInitOptions struct {
	Platforms  string
	Types      string
	Resources  string
	Repository string
	Branch     string
}

const (
	aiRetries    = 3
	aiRetryDelay = 2 * time.Second
)

func (a *AITools) say(level, text string) { a.Console.Say(level, text, AILine) }

// captured is what a script printed inside a command substitution: in text it went into the
// variable and nowhere else, and under --json the printers wrote it to stderr.
func (a *AITools) captured(level, text string) {
	if a.Console.JSON() {
		a.Console.Say(level, text, AIDecorated)
	}
}

func (a *AITools) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}

	return a.Dir + "/" + name
}

// data is the name of one of the tool's data files, inside Tool.
func (a *AITools) data(name string) string { return "data/" + name }

// readData is one of the tool's data files, or an error when the tool has no such file.
func (a *AITools) readData(name string) ([]byte, error) {
	if a.Tool == nil {
		return nil, fs.ErrNotExist
	}

	return fs.ReadFile(a.Tool, a.data(name))
}

// hasData is whether the tool has that data file, the script's `[ -f ]`.
func (a *AITools) hasData(name string) bool {
	_, err := a.readData(name)

	return err == nil
}

func (a *AITools) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}

	return time.Now()
}

// ---------------------------------------------------------------------------------- the files

// readJSON is a file of the project as load_ai_properties read it: nothing there is the empty
// object, contents that are not JSON are core.ErrAIInvalidJSON.
func (a *AITools) readJSON(name string) (any, string, error) {
	contents, err := os.ReadFile(a.path(name))
	if err != nil {
		if info, statErr := os.Stat(a.path(name)); statErr == nil && info.Mode().IsRegular() {
			return nil, "", err
		}

		return core.NewAIObject(), "{}", nil
	}

	// An empty file is valid to `jq empty`, and jq reads nothing from it
	if strings.TrimSpace(string(contents)) == "" {
		return nil, "", nil
	}

	value, err := core.ParseAIJSON(contents)

	return value, strings.TrimRight(string(contents), "\n"), err
}

// writeJSON writes a document the way the scripts did: to a temporary file beside it, made by
// mktemp and therefore readable by its owner only, and moved over the old one.
func (a *AITools) writeJSON(name string, value any) error {
	target := a.path(name)

	if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
		a.Console.Say(AIError, "Failed to create directory: "+filepath.Dir(name), AIDecorated)

		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".")
	if err != nil {
		return err
	}

	if _, err := temp.Write(core.EncodeAIJSON(value)); err != nil {
		temp.Close()           //nolint:errcheck
		os.Remove(temp.Name()) //nolint:errcheck
		a.Console.Say(AIError, "Failed to write temporary file", AIDecorated)

		return err
	}

	temp.Close() //nolint:errcheck

	if err := os.Rename(temp.Name(), target); err != nil {
		os.Remove(temp.Name()) //nolint:errcheck
		a.Console.Say(AIError, "Failed to save "+name, AIDecorated)

		return err
	}

	return nil
}

func emptyRegistration() *core.AIObject {
	registration := core.NewAIObject()
	registration.Set("skills", core.NewAIObject())
	registration.Set("agents", core.NewAIObject())

	return registration
}

// registration is load_ai_registration, inside the command substitution every caller put it in.
func (a *AITools) registration() (*core.AIObject, bool) {
	contents, err := os.ReadFile(a.path(core.AIRegistrationFile))
	if err != nil {
		return emptyRegistration(), true
	}

	// Empty is not corrupt to `jq empty`, and holds nothing
	if strings.TrimSpace(string(contents)) == "" {
		return core.NewAIObject(), true
	}

	value, err := core.ParseAIJSON(contents)
	if err != nil {
		a.captured(AIError, "Invalid JSON in "+core.AIRegistrationFile)
		a.captured(AIWarning, "Registration file is corrupted - refusing to operate")

		return nil, false
	}

	object, ok := value.(*core.AIObject)

	return object, ok
}

func (a *AITools) saveRegistration(registration *core.AIObject) bool {
	skills, _ := registration.Get("skills")
	agents, _ := registration.Get("agents")

	if !core.AITruthy(skills) || !core.AITruthy(agents) {
		a.Console.Say(AIError, "Invalid registration JSON structure (missing skills/agents keys)", AIDecorated)

		return false
	}

	return a.writeJSON(core.AIRegistrationFile, registration) == nil
}

// tracked is is_tracked: whether the registration has an entry for this path.
func (a *AITools) tracked(kind, path string) bool {
	registration, ok := a.registration()
	if !ok {
		return false
	}

	value, _ := registration.Get(kind)
	entries, _ := value.(*core.AIObject)
	entry, found := entries.Get(path)

	return found && entry != nil
}

// register is add_registration_entry: the resource's checksum and where it came from, under the
// path it was installed at.
func (a *AITools) register(kind, path string) bool {
	checksum := a.digest(path)

	registration, ok := a.registration()
	if !ok {
		return false
	}

	// An empty registration, or one missing a kind, holds nothing yet: it is started, not refused
	// for ever (deliberately unlike the script, where it blocked every registration until the file
	// was deleted by hand)
	for _, missing := range []string{"skills", "agents"} {
		if found, _ := registration.Get(missing); found == nil {
			registration.Set(missing, core.NewAIObject())
		}
	}

	value, _ := registration.Get(kind)
	entries, isObject := value.(*core.AIObject)

	if !isObject {
		return false
	}

	entries.Set(path, core.AIRegistrationEntryFor(checksum, a.now(), a.origin, a.url, a.branch, a.version))

	return a.saveRegistration(registration)
}

func (a *AITools) digest(path string) string {
	if filepath.IsAbs(path) {
		return core.DigestPath(os.DirFS("/"), strings.TrimPrefix(filepath.Clean(path), "/"))
	}

	return core.DigestPath(os.DirFS(a.Dir), filepath.Clean(path))
}

// catalog is data/ai-platforms.json; one that is not there knows no platform.
func (a *AITools) catalog() core.AIPlatformCatalog {
	contents, _ := a.readData("ai-platforms.json")

	return core.ReadAIPlatformCatalog(contents)
}

// legacy is what the registration says the tool installed in a directory a platform no longer
// reads, and that is still there, by directory in the catalog's order.
func (a *AITools) legacy() ([]core.AILegacyDirectory, map[string][]string) {
	registration, ok := a.registration()
	if !ok {
		return nil, nil
	}

	catalog := a.catalog()

	var directories []core.AILegacyDirectory

	found := map[string][]string{}

	for _, kind := range []string{"skills", "agents"} {
		value, _ := registration.Get(kind)
		entries, _ := value.(*core.AIObject)

		keys := entries.Keys()
		sort.Strings(keys)

		for _, directory := range catalog.LegacyDirectories(kind) {
			for _, path := range keys {
				if filepath.Dir(path) != filepath.Clean(directory.Directory) {
					continue
				}

				if _, err := os.Lstat(a.path(path)); err != nil {
					continue
				}

				if _, seen := found[directory.Directory]; !seen {
					directories = append(directories, directory)
				}

				found[directory.Directory] = append(found[directory.Directory], path)
			}
		}
	}

	return directories, found
}

// reportLegacy says what an earlier version of the tool left where a platform no longer looks, and
// the command that cleans it. Nothing is removed here: what is reported may be somebody's to keep.
func (a *AITools) reportLegacy(how int) {
	directories, found := a.legacy()

	end := ""
	if how != AILine {
		end = "\n"
	}

	for _, directory := range directories {
		paths := found[directory.Directory]

		a.Console.Say(AIWarning, fmt.Sprintf("%s no longer reads %s: %d item(s) an earlier %s installed there are not seen by it",
			directory.Name, directory.Directory, len(paths), a.Binary)+end, how)

		for _, path := range paths {
			a.Console.Say(AIWarning, "  "+path+end, how)
		}

		a.Console.Say(AIWarning, fmt.Sprintf("  Clean them up with: %s ai-reset && %s ai-pull", a.Binary, a.Binary)+end, how)
	}
}

// repositories is resolve_repositories: the default ones and the configuration's own after them.
// file is a list on disk; empty is the tool's own data/ai-repositories.json.
func (a *AITools) repositories(file string, config any) ([]any, bool) {
	merged := []any{}

	var (
		contents []byte
		found    bool
	)

	if file == "" {
		var err error
		contents, err = a.readData("ai-repositories.json")
		found = err == nil
	} else if info, err := os.Stat(file); err == nil && !info.IsDir() {
		contents, err = os.ReadFile(file)
		if err != nil {
			return nil, false
		}

		found = true
	}

	if found {

		value, err := core.ParseAIJSON(contents)
		if err != nil {
			return nil, false
		}

		object, _ := value.(*core.AIObject)
		listed, _ := object.Get("repositories")

		switch typed := listed.(type) {
		case []any:
			merged = append(merged, typed...)
		case nil:
		default:
			return nil, false
		}
	}

	object, _ := config.(*core.AIObject)
	custom, _ := object.Get("custom_repositories")

	switch typed := custom.(type) {
	case []any:
		merged = append(merged, typed...)
	case nil, bool:
	default:
		return nil, false
	}

	return merged, true
}

// ---------------------------------------------------------------------------------- download

func validURL(url string) bool { return strings.HasPrefix(url, "https://") }

var usualURL = regexp.MustCompile(`^https://[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

func (a *AITools) validateURL(url string) bool {
	if !validURL(url) {
		a.Console.Say(AIError, "Repository URL must use HTTPS protocol: "+url, AILine)

		return false
	}

	if !usualURL.MatchString(url) {
		a.Console.Say(AIWarning, "URL format looks unusual: "+url, AILine)
	}

	return true
}

func (a *AITools) wait() {
	a.say(AIInfo, fmt.Sprintf("Retrying in %d seconds...", int(aiRetryDelay/time.Second)))

	if a.Sleep != nil {
		a.Sleep(aiRetryDelay)
	}
}

func (a *AITools) downloadTarball(url, branch, output string) bool {
	if !a.validateURL(url) {
		return false
	}

	tarball := url + "/archive/refs/heads/" + branch + ".tar.gz"
	a.say(AIInfo, "Downloading from "+tarball+"...")

	for attempt := 1; attempt <= aiRetries; attempt++ {
		if a.Exec(false, "curl", "-fsSL", "--max-time", "60", "--retry", "2", "--retry-delay", "1",
			"-o", output, tarball) == nil {
			a.say(AIInfo, "Downloaded successfully")

			return true
		}

		a.Console.Say(AIWarning, fmt.Sprintf("Download attempt %d/%d failed", attempt, aiRetries), AILine)

		if attempt < aiRetries {
			a.wait()
		}
	}

	a.Console.Say(AIError, fmt.Sprintf("Failed to download after %d attempts", aiRetries), AILine)

	return false
}

func (a *AITools) clone(url, branch, output string) bool {
	if !a.validateURL(url) {
		return false
	}

	if a.HasGit != nil && !a.HasGit() {
		a.Console.Say(AIError, "git command not found (fallback unavailable)", AILine)

		return false
	}

	a.say(AIInfo, "Using git clone fallback for "+url+"...")

	for attempt := 1; attempt <= aiRetries; attempt++ {
		if a.Exec(true, "git", "clone", "--depth", "1", "--branch", branch, "--single-branch", url, output) == nil {
			a.say(AIInfo, "Cloned successfully")

			return true
		}

		a.Console.Say(AIWarning, fmt.Sprintf("Clone attempt %d/%d failed", attempt, aiRetries), AILine)

		if attempt < aiRetries {
			a.wait()
			os.RemoveAll(output) //nolint:errcheck
		}
	}

	a.Console.Say(AIError, fmt.Sprintf("Failed to clone after %d attempts", aiRetries), AILine)

	return false
}

// download is download_repository: the archive of the branch first, a shallow clone after.
func (a *AITools) download(url, branch, output string) bool {
	if err := os.MkdirAll(output, 0o777); err != nil {
		a.Console.Say(AIError, "Failed to create output directory: "+output, AILine)

		return false
	}

	tarball := filepath.Join(output, "repo.tar.gz")
	if temp, err := os.CreateTemp(output, "repo.*.tar.gz"); err == nil {
		tarball = temp.Name()
		temp.Close() //nolint:errcheck
	}

	if a.downloadTarball(url, branch, tarball) {
		if a.Exec(true, "tar", "-xzf", tarball, "-C", output, "--strip-components=1") == nil {
			os.Remove(tarball) //nolint:errcheck

			return true
		}

		a.Console.Say(AIWarning, "Failed to extract tarball, trying git clone...", AILine)
		os.Remove(tarball) //nolint:errcheck
	} else {
		os.Remove(tarball) //nolint:errcheck
		a.Console.Say(AIWarning, "Tarball download failed, trying git clone...", AILine)
	}

	// `rm -rf "$output"/*`: what the glob sees, which is not what starts with a dot
	for _, name := range visible(output) {
		os.RemoveAll(filepath.Join(output, name)) //nolint:errcheck
	}

	return a.clone(url, branch, output)
}

// visible is what `"$dir"/*` expands to: the names that do not start with a dot, sorted.
func visible(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}

	sort.Strings(names)

	return names
}

// validStructure is validate_repository_structure: a skills/ or an agents/ directory.
func (a *AITools) validStructure(repository string) bool {
	if !isDirectory(repository) {
		a.Console.Say(AIError, "Repository directory does not exist: "+repository, AILine)

		return false
	}

	if isDirectory(repository+"/skills") || isDirectory(repository+"/agents") {
		return true
	}

	a.Console.Say(AIWarning, "Repository has no skills/ or agents/ directories", AILine)

	return false
}

// ---------------------------------------------------------------------------------- install

// install is install_filtered, and install_from_repository when no type is asked for: the two
// said some things in different words, which is what filtered decides.
func (a *AITools) install(repository, kind, directory, types string, force bool) bool {
	filtered := types != ""

	var wanted []string
	if filtered {
		wanted = strings.Split(types, ",")

		if !a.hasData("ai-skill-types.json") {
			a.Console.Say(AIError, "Skill types configuration not found: "+a.data("ai-skill-types.json"), AILine)

			return false
		}
	}

	if !a.validStructure(repository) {
		return false
	}

	source := repository + "/" + kind
	if !isDirectory(source) {
		a.say(AIInfo, "No "+kind+" found in repository")

		return true
	}

	if kind == "agents" {
		return a.installAgents(source, directory, force)
	}

	target := a.path(directory)
	if err := os.MkdirAll(target, 0o777); err != nil {
		a.Console.Say(AIError, "Failed to create target directory: "+directory, AILine)

		return false
	}

	temp, err := os.MkdirTemp(target, ".tmp.")
	if err != nil {
		return false
	}
	defer os.RemoveAll(temp) //nolint:errcheck

	singular := strings.TrimSuffix(kind, "s")
	installed, skipped := 0, 0

	for _, name := range visible(source) {
		item := source + "/" + name
		if !isDirectory(item) {
			continue
		}

		if filtered && !core.AISkillTypeMatches(name, wanted) {
			continue
		}

		path := directory + "/" + name

		if isDirectory(a.path(path)) {
			switch {
			case a.tracked(kind, path):
				a.say(AIInfo, "Updating "+name+"...")
			case force:
				a.Console.Say(AIWarning, "Overwriting custom "+singular+": "+name+" (--force enabled)", AILine)
			default:
				if filtered {
					a.Console.Say(AIWarning, "Skipping "+name+" (custom "+singular+" exists)", AILine)
				} else {
					a.Console.Say(AIWarning, "Skipping "+name+" (custom "+singular+" exists, use --force to overwrite)", AILine)
				}

				skipped++

				continue
			}
		} else {
			a.say(AIInfo, "Installing "+name+"...")
		}

		staged := temp + "/" + name
		if copyTree(item, staged) != nil {
			a.Console.Say(AIError, "Failed to copy "+name, AILine)

			return false
		}

		if isDirectory(a.path(path)) {
			os.RemoveAll(a.path(path)) //nolint:errcheck
		}

		if os.Rename(staged, a.path(path)) != nil {
			a.Console.Say(AIError, "Failed to install "+name, AILine)

			return false
		}

		if !a.register(kind, path) {
			if filtered {
				a.Console.Say(AIWarning, "Failed to register "+name, AILine)
			} else {
				a.Console.Say(AIWarning, "Failed to register "+name+" (continuing anyway)", AILine)
			}
		}

		installed++
	}

	switch {
	case installed == 0 && skipped == 0 && filtered:
		a.say(AIInfo, "No matching "+kind+" found for types: "+types)
	case installed == 0 && skipped == 0:
		a.say(AIInfo, "No "+kind+" to install")
	default:
		a.say(AIInfo, fmt.Sprintf("Installed %d %s", installed, kind))

		if skipped > 0 {
			a.say(AIInfo, fmt.Sprintf("Skipped %d custom %s", skipped, kind))
		}
	}

	return true
}

// installAgents is install_agents_from_directory: every .md file under the source, however deep,
// installed flat by its name. Agents are not filtered by type.
func (a *AITools) installAgents(source, directory string, force bool) bool {
	target := a.path(directory)
	if err := os.MkdirAll(target, 0o777); err != nil {
		a.Console.Say(AIError, "Failed to create target directory: "+directory, AILine)

		return false
	}

	files := agentFiles(source)
	if len(files) == 0 {
		a.say(AIInfo, "No agents found in repository")

		return true
	}

	temp, err := os.MkdirTemp(target, ".tmp.")
	if err != nil {
		return false
	}
	defer os.RemoveAll(temp) //nolint:errcheck

	installed, skipped := 0, 0

	for _, file := range files {
		name := filepath.Base(file)
		if name != ".md" {
			name = strings.TrimSuffix(name, ".md")
		}

		path := directory + "/" + name + ".md"

		if isRegular(a.path(path)) {
			switch {
			case a.tracked("agents", path):
				a.say(AIInfo, "Updating "+name+"...")
			case force:
				a.Console.Say(AIWarning, "Overwriting custom agent: "+name+" (--force enabled)", AILine)
			default:
				a.Console.Say(AIWarning, "Skipping "+name+" (custom agent exists, use --force to overwrite)", AILine)

				skipped++

				continue
			}
		} else {
			a.say(AIInfo, "Installing "+name+"...")
		}

		staged := temp + "/" + name + ".md"
		if copyFile(file, staged) != nil {
			a.Console.Say(AIError, "Failed to copy "+name, AILine)

			return false
		}

		if isRegular(a.path(path)) {
			os.Remove(a.path(path)) //nolint:errcheck
		}

		if os.Rename(staged, a.path(path)) != nil {
			a.Console.Say(AIError, "Failed to install "+name, AILine)

			return false
		}

		if !a.register("agents", path) {
			a.Console.Say(AIWarning, "Failed to register "+name+" (continuing anyway)", AILine)
		}

		installed++
	}

	if installed == 0 && skipped == 0 {
		a.say(AIInfo, "No agents to install")
	} else {
		a.say(AIInfo, fmt.Sprintf("Installed %d agents", installed))

		if skipped > 0 {
			a.say(AIInfo, fmt.Sprintf("Skipped %d custom agents", skipped))
		}
	}

	return true
}

// agentFiles is `find "$dir" -type f -name "*.md"`: depth first, in the order the directory
// gives its entries — find does not sort them, so neither does this.
func agentFiles(dir string) []string {
	handle, err := os.Open(dir)
	if err != nil {
		return nil
	}

	names, _ := handle.Readdirnames(-1)
	handle.Close() //nolint:errcheck

	var files []string

	for _, name := range names {
		path := dir + "/" + name

		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		switch {
		case info.Mode().IsRegular():
			if matched, _ := filepath.Match("*.md", name); matched {
				files = append(files, path)
			}
		case info.IsDir():
			files = append(files, agentFiles(path)...)
		}
	}

	return files
}

// copyTree is `cp -r`: modes as the source's under the umask, links as links.
func copyTree(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}

		return os.Symlink(link, target)
	case info.IsDir():
		if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
			return err
		}

		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if err := copyTree(source+"/"+entry.Name(), target+"/"+entry.Name()); err != nil {
				return err
			}
		}

		return nil
	default:
		return copyFile(source, target)
	}
}

func copyFile(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck

		return err
	}

	return out.Close()
}

func isRegular(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// split is `IFS=',' read -ra`: the empty ones are looked up and find nothing, so they are left
// out here.
func split(list string) []string {
	var items []string

	for _, item := range strings.Split(list, ",") {
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

func (a *AITools) tempBase() string {
	root := a.TempRoot
	if root == "" {
		root = "/tmp"
	}

	base, err := os.MkdirTemp(root, "hm-ai-tools.")
	if err != nil {
		base, _ = os.MkdirTemp("", "hm-ai-tools.")
	}

	return base
}
