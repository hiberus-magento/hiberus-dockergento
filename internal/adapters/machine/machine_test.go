package machine

import "testing"

// The two tools that can list a port are different enough that reading one as the other is how
// the shell implementation ended up reporting a conflict as taken by a process called "LISTEN".

func TestReadingLsof(t *testing.T) {
	output := `COMMAND     PID     USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
com.docke  1234 somebody   50u  IPv6 0x1234567890abcdef      0t0  TCP *:80 (LISTEN)
com.docke  1234 somebody   52u  IPv6 0x1234567890abcdee      0t0  TCP 127.0.0.1:3306 (LISTEN)
nginx      4321 somebody    6u  IPv4 0x1234567890abcded      0t0  TCP *:8080 (LISTEN)
`

	listeners := fromLsof(output)

	if len(listeners) != 3 {
		t.Fatalf("tres puertos escuchando, y leyó %d", len(listeners))
	}

	if listeners[0].Port != "80" || listeners[0].Process != "com.docke" {
		t.Fatalf("el primero es el 80 y lo tiene Docker: %+v", listeners[0])
	}

	if listeners[1].Port != "3306" {
		t.Fatalf("una dirección concreta también es un puerto ocupado: %+v", listeners[1])
	}
}

func TestTheHeaderIsNotAListener(t *testing.T) {
	if listeners := fromLsof("COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME\n"); len(listeners) != 0 {
		t.Fatalf("la cabecera no ocupa ningún puerto: %+v", listeners)
	}
}

func TestReadingSS(t *testing.T) {
	output := `State  Recv-Q Send-Q Local Address:Port  Peer Address:Port
LISTEN 0      511          0.0.0.0:80          0.0.0.0:*
LISTEN 0      511             [::]:443            [::]:*
`

	listeners := fromSS(output)

	if len(listeners) != 2 {
		t.Fatalf("dos puertos escuchando, y leyó %d", len(listeners))
	}

	// `ss` names no process, and inventing one is worse than saying nothing: the message falls
	// back to "processes on the host", which is true
	if listeners[0].Port != "80" || listeners[0].Process != "" {
		t.Fatalf("de ss no sale ningún nombre de proceso: %+v", listeners[0])
	}

	if listeners[1].Port != "443" {
		t.Fatalf("una dirección IPv6 también es un puerto ocupado: %+v", listeners[1])
	}
}
