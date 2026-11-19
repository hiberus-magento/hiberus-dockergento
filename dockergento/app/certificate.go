package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// What ssl.sh did: a certificate for the project's domain, signed by mkcert's local authority and
// so trusted by this machine's browsers, put where the TLS terminator reads it.
//
// One thing is deliberately different. The shell generated into the directory it was run in and
// left ssl.pem there, a private key in the project's tree; here it is generated into a temporary
// directory that is removed at the end, whatever happens.
//

// certificateService is the TLS terminator, and certificateTarget where it reads its certificate.
const (
	certificateService = "hitch"
	certificateTarget  = "/etc/hitch/testcert.pem"
)

// mkcertNames is what the certificate is valid for besides the domain: every way this machine
// calls itself, so the same certificate serves whichever of them the browser was pointed at.
var mkcertNames = []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"}

// InstallCertificate generates a certificate for domain and installs it in the TLS terminator,
// then restarts it so that it is served.
//
// The terminator has to be running, and that is asked before anything happens on this machine.
// mkcert is installed when it is missing, the way the shell did. A failing step stops there with
// its error, as the shell's set -e did.
//
// restart restarts the terminator once the certificate is in. It is Compose's own restart, given
// by the caller, and not this operator's stop and up: up recreates a container whose
// configuration changed, and the terminator keeps the certificate in its filesystem, not in a
// volume, so a recreated one would have lost it. The shell restarted it the same way.
func (o Operator) InstallCertificate(project core.Project, files core.ComposeFiles, domain string, restart func() error) error {
	// From a worktree with no environment of its own the terminator is the main checkout's;
	// refused here, before anything is generated or copied into it
	if err := o.refuseFromAnUnregisteredWorktree(project, "ssl"); err != nil {
		return err
	}

	hitch, err := resolveContainer(o.Engine, project, certificateService, o.Binary)
	if err != nil {
		return err
	}

	o.announce(fmt.Sprintf("Generating SSL certificates for domain %s...\n", domain))

	work, err := os.MkdirTemp("", "hm-ssl-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck

	if err := o.ensureMkcert(work); err != nil {
		return err
	}

	o.announce("Installing SSL certificate into docker environment...\n")

	pem, err := o.generateCertificate(work, domain)
	if err != nil {
		return err
	}

	// The file transfer's target is a directory, as it is for `docker cp`: the pem is named as it
	// has to land
	if err := o.Files.Into(hitch, pem, filepath.Dir(certificateTarget)+"/"); err != nil {
		return err
	}

	if err := o.mustSucceed(project, files, certificateService,
		[]string{"chown", "hitch", certificateTarget}); err != nil {
		return err
	}

	if err := restart(); err != nil {
		return err
	}

	o.announce("SSL certificate installed. Remember to restart your browser\n")

	return nil
}

// ensureMkcert installs mkcert when it is not on the PATH, as the shell did, downloading into
// work, and refuses when it is still not there afterwards.
func (o Operator) ensureMkcert(work string) error {
	if _, err := o.Programs.LookPath("mkcert"); err == nil {
		return nil
	}

	o.announce("Required 'mkcert' command not found. Trying to install...\n")

	if err := o.installMkcert(work); err != nil {
		return err
	}

	if _, err := o.Programs.LookPath("mkcert"); err != nil {
		return core.Refusal{
			Kind:    "mkcert_missing",
			Code:    1,
			Message: "Error during 'mkcert' installation. Please do it manually and try again...",
		}
	}

	return nil
}

// installMkcert is the shell's installation, step by step: Homebrew on macOS, and on anything else
// the release binary downloaded and moved into /usr/local/bin.
func (o Operator) installMkcert(work string) error {
	if o.Platform == "mac" {
		if _, err := o.Programs.LookPath("brew"); err != nil {
			return core.Refusal{
				Kind:    "brew_missing",
				Code:    1,
				Message: "Brew is required. Please install it and try again.",
			}
		}

		return o.runAll(work,
			[]string{"brew", "install", "mkcert", "nss"},
			[]string{"mkcert", "-install"})
	}

	if err := o.runAll(work,
		[]string{"sudo", "apt-get", "-y", "install", "curl", "libnss3-tools"},
		[]string{"curl", "-JLO", "https://dl.filippo.io/mkcert/latest?for=linux/amd64"}); err != nil {
		return err
	}

	// The name the download is given carries its version, which is why the shell globbed it.
	// Unmatched, the pattern is passed as it is, as a shell without nullglob would, and the
	// command fails on it the way it failed there
	downloaded := []string{"mkcert-v*-linux-amd64"}

	if matches, err := filepath.Glob(filepath.Join(work, downloaded[0])); err == nil && len(matches) > 0 {
		downloaded = downloaded[:0]

		for _, match := range matches {
			downloaded = append(downloaded, filepath.Base(match))
		}
	}

	return o.runAll(work,
		append([]string{"chmod", "+x"}, downloaded...),
		append(append([]string{"sudo", "mv"}, downloaded...), "/usr/local/bin/mkcert"),
		[]string{"mkcert", "-install"})
}

// generateCertificate runs mkcert into work and answers the pem the terminator reads: the
// certificate followed by its key.
func (o Operator) generateCertificate(work, domain string) (string, error) {
	crt := filepath.Join(work, "ssl.crt")
	key := filepath.Join(work, "ssl.key")

	arguments := append([]string{"-cert-file", crt, "-key-file", key, domain}, mkcertNames...)
	if err := o.runAll(work, append([]string{"mkcert"}, arguments...)); err != nil {
		return "", err
	}

	certificate, err := os.ReadFile(crt)
	if err != nil {
		return "", err
	}

	private, err := os.ReadFile(key)
	if err != nil {
		return "", err
	}

	pem := filepath.Join(work, filepath.Base(certificateTarget))

	// Readable by its owner only: it carries the private key, and the chown that follows makes
	// the terminator that owner
	if err := os.WriteFile(pem, append(certificate, private...), 0o600); err != nil {
		return "", err
	}

	return pem, nil
}

// runAll runs each command on this machine in turn, and stops at the first that fails.
func (o Operator) runAll(work string, commands ...[]string) error {
	for _, command := range commands {
		if err := o.Programs.Run(work, command[0], command[1:]...); err != nil {
			return core.Refusal{
				Kind:    "host_command_failed",
				Code:    1,
				Message: fmt.Sprintf("'%s' failed: %v", strings.Join(command, " "), err),
			}
		}
	}

	return nil
}
