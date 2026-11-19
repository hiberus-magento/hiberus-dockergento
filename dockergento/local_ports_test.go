package dockergento

import (
	"net"
	"strconv"
	"testing"
)

// The two halves of a tunnel's readiness check that live on this machine: a free port nothing
// answers on, and seeing something answer once it does.
func TestAFreeLocalPortAnswersOnlyOnceSomethingListensOnIt(t *testing.T) {
	port, err := freeLocalPort()
	if err != nil || port <= 0 {
		t.Fatalf("freeLocalPort() = %d, %v", port, err)
	}

	if answersLocally(port) {
		t.Fatalf("port %d answered before anything listened on it", port)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("the free port %d could not be listened on: %v", port, err)
	}
	defer listener.Close()

	if !answersLocally(port) {
		t.Errorf("port %d did not answer while listened on", port)
	}
}
