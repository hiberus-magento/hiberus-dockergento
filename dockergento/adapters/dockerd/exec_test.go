package dockerd

import "testing"

func TestElUsuarioDelRunnerLlegaAlExec(t *testing.T) {
	options := Runner{User: "root"}.options([]string{"id"}, nil)

	if options.User != "root" {
		t.Fatalf("exec user = %q, want root", options.User)
	}
}

func TestSinUsuarioElExecEsDelUsuarioDelContenedor(t *testing.T) {
	options := Runner{}.options([]string{"id"}, nil)

	if options.User != "" {
		t.Fatalf("exec user = %q, want the container's default", options.User)
	}
}
