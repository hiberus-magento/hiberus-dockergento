package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// The Magento Cloud defaults transfer-media.sh offered: only the user has none.
const (
	defaultMediaHost = "ssh.eu-3.magento.cloud"
	defaultMediaPath = "/app/pub/media"
)

// runRsync runs rsync attached to this terminal, so that SSH can ask for a passphrase. A package
// variable so that no test ever connects anywhere.
var runRsync = func(args []string) error {
	command := exec.Command("rsync", args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr

	return command.Run()
}

// lookRsync says whether rsync is on this machine.
var lookRsync = func() error {
	_, err := exec.LookPath("rsync")

	return err
}

// transferMedia copies a remote store's pub/media into ./pub with rsync over SSH, keeping what is
// already here and leaving the caches behind — transfer-media.sh, questions and messages included.
func transferMedia(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	host, user, path := defaultMediaHost, "", defaultMediaPath

	for _, argument := range args {
		switch {
		case strings.HasPrefix(argument, "--ssh-host="):
			host = strings.TrimPrefix(argument, "--ssh-host=")
		case strings.HasPrefix(argument, "--ssh-user="):
			user = strings.TrimPrefix(argument, "--ssh-user=")
		case strings.HasPrefix(argument, "--ssh-path="):
			path = strings.TrimPrefix(argument, "--ssh-path=")
		default:
			return failure(stderr, jsonOutput, "transfer-media", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument),
				binaryName()+" transfer-media --ssh-user=<user> [--ssh-host=<host>] [--ssh-path=<path>]")
		}
	}

	if _, code := projectOr(stderr, jsonOutput, "transfer-media"); code != 0 {
		return code
	}

	fmt.Fprint(stdout, "Media transfer assistant:\n")

	// ask answers with the suggestion when nobody is there to ask, which is what the shell's
	// non-interactive path did with its defaults
	host, _ = ask(fmt.Sprintf("SSH Host [Default: '%s']:", host), host)
	user, _ = ask(fmt.Sprintf("SSH User [Default: '%s']:", user), user)
	path, _ = ask(fmt.Sprintf("SSH pub/media path [Default: '%s']:", path), path)

	if host == "" || user == "" || path == "" {
		return failure(stderr, jsonOutput, "transfer-media", exitUsage, "input_required",
			"Missing SSH connection data", binaryName()+" transfer-media --ssh-user=<user>")
	}

	fmt.Fprintf(stdout, "You are going to transfer files from [%s] to [LOCALHOST].\n", host)

	if os.Getenv("HM_NON_INTERACTIVE") == "" {
		ask("Press enter to continue...", "") //nolint:errcheck
	}

	if err := lookRsync(); err != nil {
		return failure(stderr, jsonOutput, "transfer-media", exitError, "rsync_missing",
			"Rsync command not available", "")
	}

	if info, err := os.Stat("pub/media"); err != nil || !info.IsDir() {
		return failure(stderr, jsonOutput, "transfer-media", exitError, "no_media_directory",
			"Local pub/media directory not found. Are you in the correct path?", "")
	}

	fmt.Fprintf(stdout, "Transferring media files from [%s] to [LOCALHOST]...\n", host)

	if err := runRsync([]string{"-az", "--ignore-existing", "--exclude", "**/cache",
		user + "@" + host + ":" + path, "./pub"}); err != nil {
		return failure(stderr, jsonOutput, "transfer-media", exitError, "transfer_failed",
			"The media transfer did not finish: "+err.Error(), "")
	}

	fmt.Fprint(stdout, good("All media content transferred!\n"))

	return exitOK
}
