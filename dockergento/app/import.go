package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

// Importer replaces the contents of a project's database, and everything that has to happen
// around that.
//
// It is not the database's job: it runs the anonymiser, which is a container of its own, and then
// Magento's own CLI to point the store at this machine. Those are three different things that
// have to happen in one order, which is what a use case is for.
type Importer struct {
	Database     Database
	Orchestrator ports.Orchestrator
	State        ports.DataState
	Properties   ports.Properties
	OneOff       ports.OneOff

	// Settings is the tool's own local_settings.json, copied into a project that has none.
	Settings string

	// Progress and Announce are how the long parts say what they are doing.
	Progress func(label string) func(ok bool, note string)
	Announce func(string)

	// Errors is where the client's own complaints go. A dump that half applies says why there.
	Errors io.Writer

	// Binary is the name the tool was invoked as, for the hints.
	Binary string
}

// ImportOptions is what an import was asked to do beyond importing.
type ImportOptions struct {
	// File is the dump.
	File string

	// CleanDefiners strips the DEFINER clauses. A dump taken as one database user and restored as
	// another fails on every view and trigger it defines, and the error names a user rather than
	// the problem.
	CleanDefiners bool

	// Anonymise replaces the personal data once the dump is in.
	Anonymise bool
}

// definer matches what has to go: `DEFINER=`user`@`host“ up to the comment that closes the
// clause. The same expression the shell implementation used, kept because it is the one that has
// been run against real dumps.
var definer = regexp.MustCompile(`DEFINER=[^*]*\*`)

// Import replaces the contents of the project's database with a dump.
//
// The order is not arbitrary. The record of the data having been anonymised is cleared as soon as
// the new contents are in and before anything else can fail: whatever the dump brought, nobody
// anonymised it, and a reassuring "yes" left over from before an import is worse than no record
// at all.
func (d Importer) Import(project core.Project, files core.ComposeFiles, options ImportOptions) error {
	dump, err := os.Open(options.File)
	if err != nil {
		return fmt.Errorf("no such file: %s", options.File)
	}
	defer dump.Close()

	label := "Importing the database..."
	var contents io.Reader = dump

	if options.CleanDefiners {
		label = "Importing the database, with the DEFINER clauses removed..."

		// Streamed rather than written to a second file beside the first. The shell
		// implementation left a `-cleaned.sql` next to the user's dump and never removed it,
		// which on a real Magento dump is another gigabyte
		contents = withoutDefiners(dump)
	}

	//
	// The domain, before the import replaces the row it is read from.
	//
	// Only when the project does not have one: it is written into the project's properties, and
	// a project that already declared its domain has said what it wants.
	//
	if project.Domain == "" {
		d.rememberDomain(project)
	}

	done := d.begin(label)

	status, err := d.Database.Feed(project, contents, d.Errors)
	if err != nil {
		done(false, "")

		return err
	}

	done(status == 0, "")

	if status != 0 {
		return core.Refusal{
			Kind:    "import_failed",
			Code:    1,
			Message: "The dump could not be imported",
			Hint:    "Check that the file is a SQL dump this database can read",
		}
	}

	if err := d.State.Clear(project.Name); err != nil {
		return err
	}

	if options.Anonymise {
		if err := d.Anonymise(project); err != nil {
			return err
		}
	}

	return d.configure(project, files)
}

// withoutDefiners strips the clauses as the dump is read, so a gigabyte never has to exist twice.
func withoutDefiners(dump io.Reader) io.Reader {
	read, write := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(dump)

		// A dump is one statement per line, and a Magento one has lines far longer than the
		// default limit
		scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

		for scanner.Scan() {
			if _, err := fmt.Fprintln(write, definer.ReplaceAllString(scanner.Text(), "*")); err != nil {
				write.CloseWithError(err) //nolint:errcheck

				return
			}
		}

		write.CloseWithError(scanner.Err()) //nolint:errcheck
	}()

	return read
}

// rememberDomain reads the storefront's address out of the database and writes it into the
// project, for a project that never declared one.
//
// Read before the import because the import replaces the row. A project with no domain is one
// nothing can be reached at, and asking the database it came from is better than asking a person
// who would have to look it up in the same place.
func (d Importer) rememberDomain(project core.Project) {
	var tables strings.Builder

	if _, err := d.Database.Query(project, "SHOW TABLES LIKE 'core_config_data';", &tables); err != nil {
		return
	}

	if strings.TrimSpace(tables.String()) == "" {
		return
	}

	var url strings.Builder

	if _, err := d.Database.Query(project,
		"SELECT value FROM core_config_data WHERE path='web/secure/base_url';", &url); err != nil {
		return
	}

	domain := hostOf(url.String())
	if domain == "" {
		return
	}

	d.Properties.Set(project.Root, "DOMAIN", domain) //nolint:errcheck
}

// hostOf takes the host out of the first address in what the client printed, whose first line is
// the column name.
func hostOf(answer string) string {
	for _, line := range strings.Split(strings.TrimSpace(answer), "\n") {
		line = strings.TrimSpace(line)

		at := strings.Index(line, "://")
		if at < 0 {
			continue
		}

		rest := line[at+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[:slash]
		}

		return rest
	}

	return ""
}

func (d Importer) begin(label string) func(bool, string) {
	if d.Progress == nil {
		return func(bool, string) {}
	}

	return d.Progress(label)
}
