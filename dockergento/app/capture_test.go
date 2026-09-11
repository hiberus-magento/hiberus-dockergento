package app

import (
	"bytes"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

func capturadora(containers []core.Container, corredor *runner) Capture {
	return Capture{Engine: &engine{containers: containers}, Runner: corredor, Binary: "hm"}
}

func TestLaCapturaLlegaAlContenedorDeEsteProyectoParaCualquierServicio(t *testing.T) {
	// La resolución no es un segundo `db` disfrazado: tiene que llegar al servicio que se le pida,
	// aunque haya varios proyectos en la máquina.
	corredor := &runner{answer: "listo\n"}
	captura := capturadora([]core.Container{
		{ID: "otro", Project: "otra-tienda", ComposeService: "phpfpm", Running: true},
		{ID: "abc123", Project: "tienda", ComposeService: "phpfpm", Running: true},
	}, corredor)

	if _, _, err := captura.Run(core.Project{Name: "tienda"}, "phpfpm", []string{"echo", "hola"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run = %v, want no error", err)
	}

	if corredor.container != "abc123" {
		t.Fatalf("container = %q, want this project's phpfpm", corredor.container)
	}
}

func TestSinElServicioArribaSeDiceAsiNombrandoloAEl(t *testing.T) {
	// Probando que el ayudante está de verdad parametrizado y no es una segunda copia con "db"
	// grabado a fuego: el servicio y la pista tienen que nombrar "phpfpm", no "db".
	captura := capturadora([]core.Container{
		{ID: "abc123", Project: "tienda", ComposeService: "phpfpm", Running: false},
	}, &runner{})

	_, _, err := captura.Run(core.Project{Name: "tienda"}, "phpfpm", []string{"echo", "hola"}, &bytes.Buffer{})

	refusal := refusalOf(t, err)

	if refusal.Code != 5 || refusal.Hint != "hm start phpfpm" {
		t.Fatalf("refusal = %+v, want code 5 and a hint naming phpfpm", refusal)
	}
}

func TestLaSalidaEstandarVuelveComoElTextoDevuelto(t *testing.T) {
	corredor := &runner{answer: "1\n"}
	captura := capturadora([]core.Container{{ID: "abc123", Project: "tienda", ComposeService: "phpfpm", Running: true}}, corredor)

	salida, _, err := captura.Run(core.Project{Name: "tienda"}, "phpfpm", []string{"echo", "1"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run = %v, want no error", err)
	}

	if salida != "1\n" {
		t.Fatalf("output = %q, want what the command printed, unchanged", salida)
	}
}

func TestLasQuejasNoLleganAlTextoDevuelto(t *testing.T) {
	// Lo que se escribe en el error stream no puede terminar dentro de una copia que se está
	// escribiendo en un fichero.
	corredor := &runner{answer: "1\n", complaint: "warning: something\n"}
	captura := capturadora([]core.Container{{ID: "abc123", Project: "tienda", ComposeService: "phpfpm", Running: true}}, corredor)

	var quejas bytes.Buffer

	salida, _, err := captura.Run(core.Project{Name: "tienda"}, "phpfpm", []string{"echo", "1"}, &quejas)
	if err != nil {
		t.Fatalf("Run = %v, want no error", err)
	}

	if quejas.String() != "warning: something\n" {
		t.Fatalf("errors = %q, want the complaint written through", quejas.String())
	}

	if salida != "1\n" {
		t.Fatalf("output = %q, want the complaint absent from it", salida)
	}
}

func TestElCodigoDeSalidaDeLaCapturaEsElDelComando(t *testing.T) {
	corredor := &runner{status: 1}
	captura := capturadora([]core.Container{{ID: "abc123", Project: "tienda", ComposeService: "phpfpm", Running: true}}, corredor)

	_, status, err := captura.Run(core.Project{Name: "tienda"}, "phpfpm", []string{"false"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run(a command that fails) = %v, want no error of our own", err)
	}

	if status != 1 {
		t.Fatalf("status = %d, want the command's own", status)
	}
}
