package app

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

type runner struct {
	container   string
	command     []string
	environment []string
	answer      string
	fed         string
	complaint   string
	status      int
}

func (r *runner) Run(container string, command, environment []string, out io.Writer) (int, error) {
	r.container, r.command, r.environment = container, command, environment

	out.Write([]byte(r.answer)) //nolint:errcheck

	return r.status, nil
}

// Capture keeps the two streams apart, which is the whole reason it exists: what is written to
// the error stream must not end up inside a copy being written to a file.
func (r *runner) Capture(container string, command []string, out, errors io.Writer) (int, error) {
	r.container, r.command = container, command

	out.Write([]byte(r.answer))       //nolint:errcheck
	errors.Write([]byte(r.complaint)) //nolint:errcheck

	return r.status, nil
}

func (r *runner) Feed(container string, command []string, in io.Reader, out io.Writer) (int, error) {
	r.container, r.command = container, command

	fed, _ := io.ReadAll(in)
	r.fed = string(fed)

	return r.status, nil
}

func baseDeDatos(containers []core.Container, corredor *runner) Database {
	return Database{Engine: engine{containers: containers}, Runner: corredor, Binary: "hm"}
}

func contenedorDeDatos(project string, running bool) core.Container {
	return core.Container{
		ID: "abc123", Project: project, ComposeService: "db", Running: running,
	}
}

func TestLaConsultaLlegaAlContenedorDeEsteProyecto(t *testing.T) {
	//
	// `docker ps -f name=db` casa por subcadena contra todos los proyectos de la máquina: con dos
	// entornos arriba podría devolver varios ids, o el de otro proyecto. Se busca por etiqueta.
	//
	corredor := &runner{answer: "1\n"}
	db := baseDeDatos([]core.Container{
		{ID: "otro", Project: "otra-tienda", ComposeService: "db", Running: true},
		contenedorDeDatos("tienda", true),
	}, corredor)

	var salida bytes.Buffer

	if _, err := db.Query(core.Project{Name: "tienda"}, "SELECT 1", &salida); err != nil {
		t.Fatalf("consulta fallida: %v", err)
	}

	if corredor.container != "abc123" {
		t.Fatalf("fue al contenedor equivocado: %q", corredor.container)
	}

	if salida.String() != "1\n" {
		t.Fatalf("la respuesta se devuelve tal cual: %q", salida.String())
	}
}

func TestLaSentenciaViajaEnElEntornoYNoEnElComando(t *testing.T) {
	// Una consulta va llena de comillas y acentos graves; pasarla como argumento a través de un
	// shell es un fallo de comillado esperando a la consulta adecuada
	corredor := &runner{}
	db := baseDeDatos([]core.Container{contenedorDeDatos("tienda", true)}, corredor)

	sentencia := `SELECT 'con "comillas" y ` + "`acentos`" + `'`

	db.Query(core.Project{Name: "tienda"}, sentencia, &bytes.Buffer{}) //nolint:errcheck

	if len(corredor.environment) != 1 || corredor.environment[0] != "QUERY="+sentencia {
		t.Fatalf("la sentencia tiene que ir en el entorno: %v", corredor.environment)
	}

	for _, parte := range corredor.command {
		if strings.Contains(parte, "comillas") {
			t.Fatalf("y no en el comando: %v", corredor.command)
		}
	}
}

func TestElClienteSeResuelveDentroDelContenedor(t *testing.T) {
	// MariaDB 11 quitó el nombre `mysql`, y las imágenes que se usan están a los dos lados de ese
	// cambio: 10.2 sólo tiene `mysql`, 11 sólo `mariadb`
	corredor := &runner{}
	db := baseDeDatos([]core.Container{contenedorDeDatos("tienda", true)}, corredor)

	db.Query(core.Project{Name: "tienda"}, "SELECT 1", &bytes.Buffer{}) //nolint:errcheck

	entero := strings.Join(corredor.command, " ")

	if !strings.Contains(entero, "command -v mariadb") || !strings.Contains(entero, "command -v mysql") {
		t.Fatalf("tiene que probar los dos nombres: %q", entero)
	}
}

func TestSinBaseDeDatosArribaSeDiceAsi(t *testing.T) {
	// Cualquier otra respuesta sería un error de conexión tres capas más abajo
	db := baseDeDatos([]core.Container{contenedorDeDatos("tienda", false)}, &runner{})

	_, err := db.Query(core.Project{Name: "tienda"}, "SELECT 1", &bytes.Buffer{})

	refusal := refusalOf(t, err)

	if refusal.Code != 5 || refusal.Hint != "hm start db" {
		t.Fatalf("con su código y con qué hacer: %+v", refusal)
	}
}

func TestElCodigoDeSalidaEsElDeLaConsulta(t *testing.T) {
	corredor := &runner{status: 1}
	db := baseDeDatos([]core.Container{contenedorDeDatos("tienda", true)}, corredor)

	status, err := db.Query(core.Project{Name: "tienda"}, "SELECT * FROM no_existe", &bytes.Buffer{})
	if err != nil {
		t.Fatalf("una consulta que falla no es un fallo de la herramienta: %v", err)
	}

	if status != 1 {
		t.Fatalf("el código es el del cliente: %d", status)
	}
}
