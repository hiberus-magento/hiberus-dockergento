package core

import "testing"

func TestUnaRamaSeConvierteEnAlgoUsable(t *testing.T) {
	// Acaba siendo nombre de host, de volumen y de fichero
	casos := map[string]string{
		"feature/checkout": "feature-checkout",
		"Feature_X":        "feature-x",
		"feature/x":        "feature-x",
		"release/2.0.0":    "release-2-0-0",
		"--raro--":         "raro",
		"con espacios":     "con-espacios",
	}

	for rama, esperado := range casos {
		if Slug(rama) != esperado {
			t.Errorf("%q tenía que dar %q y dio %q", rama, esperado, Slug(rama))
		}
	}
}

func TestDosRamasDistintasPuedenReducirseAlMismoNombre(t *testing.T) {
	// Y por eso el comando lo rechaza en vez de inventar un nombre: aquí el nombre decide qué
	// contenedores, qué volúmenes y qué base de datos se usan
	if Slug("feature/x") != Slug("Feature_X") {
		t.Fatal("son el mismo nombre, y el comando tiene que darse cuenta")
	}
}

func TestUnaRamaQueNoDejaNada(t *testing.T) {
	if Slug("///") != "" {
		t.Fatalf("no queda nada usable: %q", Slug("///"))
	}
}

func TestLosPerfiles(t *testing.T) {
	if _, ok := ProfileKeeps("inventado"); ok {
		t.Fatal("un perfil que no existe no existe")
	}

	keeps, ok := ProfileKeeps("agent")
	if !ok || len(keeps) != 5 {
		t.Fatalf("el perfil de agente lleva php, nginx, base, búsqueda y redis: %v", keeps)
	}

	keeps, ok = ProfileKeeps("full")
	if !ok || len(keeps) != 0 {
		t.Fatalf("`full` es todo lo que el proyecto tenga: %v", keeps)
	}
}

func TestPorDondeSeLlegaACadaPerfil(t *testing.T) {
	// `full` lleva Varnish delante; los demás llegan directos a nginx
	servicio, puerto := WebService("full")
	if servicio != "varnish" || puerto != "6081" {
		t.Fatalf("con todo el stack se entra por Varnish: %s:%s", servicio, puerto)
	}

	servicio, _ = WebService("agent")
	if servicio != "nginx" {
		t.Fatalf("sin Varnish se entra por nginx: %s", servicio)
	}
}
