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
			t.Errorf("Slug(%q) = %q, want %q", rama, Slug(rama), esperado)
		}
	}
}

func TestDosRamasDistintasPuedenReducirseAlMismoNombre(t *testing.T) {
	// Y por eso el comando lo rechaza en vez de inventar un nombre: aquí el nombre decide qué
	// contenedores, qué volúmenes y qué base de datos se usan
	if Slug("feature/x") != Slug("Feature_X") {
		t.Fatal("two branches slugged to different names, want the collision to show")
	}
}

func TestUnaRamaQueNoDejaNada(t *testing.T) {
	if Slug("///") != "" {
		t.Fatalf("Slug(\"///\") = %q, want nothing usable", Slug("///"))
	}
}

func TestLosPerfiles(t *testing.T) {
	if _, ok := ProfileKeeps("inventado"); ok {
		t.Fatal("ProfileKeeps(a profile that is not one) succeeded, want a refusal")
	}

	keeps, ok := ProfileKeeps("agent")
	if !ok || len(keeps) != 5 {
		t.Fatalf("ProfileKeeps(agent) = %v, want php, nginx, db, search and redis", keeps)
	}

	keeps, ok = ProfileKeeps("full")
	if !ok || len(keeps) != 0 {
		t.Fatalf("ProfileKeeps(full) = %v, want nothing, which means everything", keeps)
	}
}

func TestPorDondeSeLlegaACadaPerfil(t *testing.T) {
	// `full` lleva Varnish delante; los demás llegan directos a nginx
	servicio, puerto := WebService("full")
	if servicio != "varnish" || puerto != "6081" {
		t.Fatalf("WebService(full) = %s:%s, want varnish", servicio, puerto)
	}

	servicio, _ = WebService("agent")
	if servicio != "nginx" {
		t.Fatalf("WebService(a profile without varnish) = %s, want nginx", servicio)
	}
}
