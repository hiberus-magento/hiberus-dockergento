package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/adapters/fsprops"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

//
// The DEFINER clauses, which are the reason `-d` exists.
//
// A dump taken as one database user and restored as another fails on every view and trigger it
// defines, and the error names a user rather than the problem.
//

func limpiado(t *testing.T, volcado string) string {
	t.Helper()

	limpio, err := io.ReadAll(withoutDefiners(strings.NewReader(volcado)))
	if err != nil {
		t.Fatalf("stripping the definers = %v, want no error", err)
	}

	return string(limpio)
}

func TestLaClausulaSeQuitaYElComentarioSeQueda(t *testing.T) {
	// La forma que produce mysqldump: la cláusula va en su propio comentario versionado
	linea := "/*!50001 CREATE*/ /*!50013 DEFINER=`alguien`@`otro-sitio` SQL SECURITY DEFINER*/ " +
		"/*!50001 VIEW `vista` AS SELECT 1 */;"

	limpio := limpiado(t, linea)

	if strings.Contains(limpio, "alguien") {
		t.Fatalf("stripped dump = %q, want the definer gone", limpio)
	}

	if !strings.Contains(limpio, "VIEW `vista`") {
		t.Fatalf("stripped dump = %q, want the view kept", limpio)
	}
}

func TestLoQueNoTieneClausulaNoSeToca(t *testing.T) {
	volcado := "CREATE TABLE cosas (id INT);\nINSERT INTO cosas VALUES (1);\n"

	if limpiado(t, volcado) != volcado {
		t.Fatalf("a dump with no definers = %q, want it unchanged", limpiado(t, volcado))
	}
}

func TestUnVolcadoGrandeNoRompeElLector(t *testing.T) {
	// Una línea de un volcado de Magento pasa de largo el límite por defecto del lector
	larga := "INSERT INTO cosas VALUES ('" + strings.Repeat("x", 2*1024*1024) + "');"

	if len(limpiado(t, larga)) < len(larga) {
		t.Fatal("a line longer than the buffer was lost, want it carried through")
	}
}

//
// El dominio se saca de la dirección que la tienda tiene guardada, cuyo primer renglón es el
// nombre de la columna.
//

func TestElDominioSaleDeLaDireccion(t *testing.T) {
	casos := map[string]string{
		"value\nhttps://tienda.test/\n":    "tienda.test",
		"value\nhttp://otra.local/ruta/\n": "otra.local",
		"value\n":                          "",
		"":                                 "",
		"value\nno-es-una-direccion\n":     "",
	}

	for respuesta, esperado := range casos {
		if hostOf(respuesta) != esperado {
			t.Errorf("hostOf(%q) = %q, want %q", respuesta, hostOf(respuesta), esperado)
		}
	}
}

//
// El dominio que se recuerda al importar.
//
// Un proyecto sin dominio no se alcanza en ningún sitio, y la base de datos de la que viene ya
// sabe cuál era. Se escribe en el properties.json del proyecto con el lector de verdad, porque lo
// que se fija aquí es lo que queda en el fichero, no qué llamadas se hicieron.
//

// porConsulta contesta a cada sentencia lo suyo: comprobar la tabla y leer la dirección son dos
// preguntas distintas, y una respuesta única para las dos escondería cuál de ellas se hizo.
type porConsulta struct{ respuestas map[string]string }

func (r *porConsulta) Run(_ string, _, environment []string, out io.Writer) (int, error) {
	for _, variable := range environment {
		for trozo, respuesta := range r.respuestas {
			if strings.HasPrefix(variable, "QUERY=") && strings.Contains(variable, trozo) {
				out.Write([]byte(respuesta)) //nolint:errcheck
			}
		}
	}

	return 0, nil
}

func (r *porConsulta) Feed(string, []string, io.Reader, io.Writer) (int, error) { return 0, nil }

func (r *porConsulta) Capture(string, []string, io.Writer, io.Writer) (int, error) { return 0, nil }

func importadorConDireccion(direccion string) Importer {
	corredor := &porConsulta{respuestas: map[string]string{
		"SHOW TABLES":         "Tables_in_magento (core_config_data)\ncore_config_data\n",
		"web/secure/base_url": "value\n" + direccion + "\n",
	}}

	return Importer{
		Database: Database{
			Engine: &engine{containers: []core.Container{contenedorDeDatos("tienda", true)}}, Runner: corredor, Binary: "hm",
		},
		Properties: fsprops.Reader{},
	}
}

func propiedadesEnDisco(t *testing.T, raiz string) map[string]any {
	t.Helper()

	contenido, err := os.ReadFile(filepath.Join(raiz, "config", "docker", "properties.json"))
	if err != nil {
		t.Fatalf("reading properties.json = %v, want it written", err)
	}

	propiedades := map[string]any{}
	if err := json.Unmarshal(contenido, &propiedades); err != nil {
		t.Fatalf("properties.json = %q, want valid JSON: %v", contenido, err)
	}

	return propiedades
}

func TestElDominioDeLaBaseDeDatosQuedaEscritoEnLasPropiedades(t *testing.T) {
	raiz := t.TempDir()

	importadorConDireccion("https://recuperado.test/").rememberDomain(core.Project{Name: "tienda", Root: raiz})

	if dominio := propiedadesEnDisco(t, raiz)["DOMAIN"]; dominio != "recuperado.test" {
		t.Fatalf("DOMAIN = %v, want the host of the stored base_url", dominio)
	}
}

func TestRecordarElDominioNoBorraLoQueElProyectoYaTenia(t *testing.T) {
	// El fichero va en el repositorio: rehacerlo con la única clave que se estaba mirando es como
	// un proyecto perdía su nombre, y con él sus contenedores y sus volúmenes
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "config", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(raiz, "config", "docker", "properties.json"),
		[]byte(`{"COMPOSE_PROJECT_NAME": "principal", "MAGENTO_DIR": "./src"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	importadorConDireccion("https://recuperado.test/").rememberDomain(core.Project{Name: "tienda", Root: raiz})

	propiedades := propiedadesEnDisco(t, raiz)

	if propiedades["COMPOSE_PROJECT_NAME"] != "principal" || propiedades["MAGENTO_DIR"] != "./src" {
		t.Fatalf("properties = %v, want the existing keys kept", propiedades)
	}

	if propiedades["DOMAIN"] != "recuperado.test" {
		t.Fatalf("DOMAIN = %v, want the recovered domain beside them", propiedades["DOMAIN"])
	}
}
