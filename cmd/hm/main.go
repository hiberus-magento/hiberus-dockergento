// Command hm is the entry point of Hiberus Dockergento.
//
// It is the Go implementation, and for everything not ported yet it runs the shell one. The
// substitution is meant to be invisible: same commands, same output, same exit codes.
package main

import (
	"os"

	"github.com/hiberus-magento/hiberus-dockergento/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
