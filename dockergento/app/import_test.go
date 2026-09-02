package app

import (
	"io"
	"strings"
	"testing"
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
		t.Fatalf("no se pudo limpiar: %v", err)
	}

	return string(limpio)
}

func TestLaClausulaSeQuitaYElComentarioSeQueda(t *testing.T) {
	// La forma que produce mysqldump: la cláusula va en su propio comentario versionado
	linea := "/*!50001 CREATE*/ /*!50013 DEFINER=`alguien`@`otro-sitio` SQL SECURITY DEFINER*/ " +
		"/*!50001 VIEW `vista` AS SELECT 1 */;"

	limpio := limpiado(t, linea)

	if strings.Contains(limpio, "alguien") {
		t.Fatalf("el usuario ajeno tenía que desaparecer: %q", limpio)
	}

	if !strings.Contains(limpio, "VIEW `vista`") {
		t.Fatalf("y la vista tenía que quedarse: %q", limpio)
	}
}

func TestLoQueNoTieneClausulaNoSeToca(t *testing.T) {
	volcado := "CREATE TABLE cosas (id INT);\nINSERT INTO cosas VALUES (1);\n"

	if limpiado(t, volcado) != volcado {
		t.Fatalf("un volcado sin cláusulas sale como entró: %q", limpiado(t, volcado))
	}
}

func TestUnVolcadoGrandeNoRompeElLector(t *testing.T) {
	// Una línea de un volcado de Magento pasa de largo el límite por defecto del lector
	larga := "INSERT INTO cosas VALUES ('" + strings.Repeat("x", 2*1024*1024) + "');"

	if len(limpiado(t, larga)) < len(larga) {
		t.Fatal("la línea larga se quedó por el camino")
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
			t.Errorf("de %q se esperaba %q y salió %q", respuesta, esperado, hostOf(respuesta))
		}
	}
}
