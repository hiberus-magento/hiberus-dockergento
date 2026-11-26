package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// transferLog is the record of one transfer-db: the steps, the commands, what each said and how
// it ended. A transfer that stopped halfway used to leave nothing behind to read; this is what is
// left now, and every failure points at it.
//
// It never holds the remote password: everything written goes through hide first, the commands
// and what the remote side said alike. A transfer whose log cannot be opened still runs — the
// log is for afterwards, not a condition — and hide still applies to what reaches the terminal.
type transferLog struct {
	file *os.File
	path string
	hide *strings.Replacer
}

// openTransferLog starts a log for project's transfer, hiding the secrets wherever they appear.
// A secret also appears escaped for single quotes, the way it goes into the dump's command line,
// and that form is replaced first: it contains the plain one.
func openTransferLog(project string, secrets ...string) (*transferLog, error) {
	pairs := []string{}

	for _, secret := range secrets {
		if secret != "" {
			pairs = append(pairs, insideQuotes(secret), "***", secret, "***")
		}
	}

	log := &transferLog{hide: strings.NewReplacer(pairs...)}

	directory := transferLogDir()
	if err := os.MkdirAll(directory, 0o755); err != nil { //nolint:gosec
		return log, err
	}

	started := time.Now()
	name := "transfer-db-" + project + "-" + started.Format("20060102-150405")

	// Two transfers in the same second get two logs, never one log with both
	for attempt := 1; ; attempt++ {
		path := filepath.Join(directory, name+".log")
		if attempt > 1 {
			path = filepath.Join(directory, fmt.Sprintf("%s-%d.log", name, attempt))
		}

		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec
		if errors.Is(err, os.ErrExist) && attempt < 100 {
			continue
		}

		if err != nil {
			return log, err
		}

		log.file, log.path = file, path

		break
	}

	log.printf("%s transfer-db, %s\n", binaryName(), started.Format(time.RFC3339))

	return log, nil
}

// transferLogDir is beside the rest of what hm keeps on this machine: ~/.hm/logs, or next to
// HM_STATE_DIR when that moves the state elsewhere, as the registry does.
func transferLogDir() string {
	if state := os.Getenv("HM_STATE_DIR"); state != "" {
		return filepath.Join(filepath.Dir(state), "logs")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "hm-logs")
	}

	return filepath.Join(home, ".hm", "logs")
}

// redact is text without the secrets, for the log and for the terminal.
func (l *transferLog) redact(text string) string {
	return l.hide.Replace(text)
}

func (l *transferLog) printf(format string, args ...any) {
	if l.file == nil {
		return
	}

	fmt.Fprint(l.file, l.redact(fmt.Sprintf(format, args...)))
}

// step starts a step: when, what, and the command it runs.
func (l *transferLog) step(name, command string) {
	l.printf("\n[%s] %s\n", time.Now().Format("15:04:05"), name)

	if command != "" {
		l.printf("$ %s\n", command)
	}
}

// ended is what a step said and the status it ended with.
func (l *transferLog) ended(said string, status int) {
	if said != "" {
		l.printf("%s", strings.TrimRight(said, "\n")+"\n")
	}

	l.printf("exit status %d\n", status)
}

// where is the line that ends every failure: where the whole story is.
func (l *transferLog) where() string {
	if l.path == "" {
		return ""
	}

	return "\nThe whole transfer is logged in " + l.path
}

func (l *transferLog) close() {
	if l.file != nil {
		l.file.Close()
	}
}
