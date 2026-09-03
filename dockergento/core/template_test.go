package core

import "testing"

func TestUnaDireccionDeDosMitades(t *testing.T) {
	// `<nombre>` es de este proyecto; `<proyecto>/<nombre>` es de otro, que es lo que permite
	// levantar un entorno a partir de los datos de un compañero
	proyecto, nombre := ParseTemplate("base", "tienda")
	if proyecto != "tienda" || nombre != "base" {
		t.Fatalf("ParseTemplate(name) = %q, %q, want this project and the name", proyecto, nombre)
	}

	proyecto, nombre = ParseTemplate("otra/base", "tienda")
	if proyecto != "otra" || nombre != "base" {
		t.Fatalf("ParseTemplate(project/name) = %q, %q, want the project it names", proyecto, nombre)
	}
}

func TestElNombreAcabaEnUnVolumen(t *testing.T) {
	// Acaba en el nombre de un volumen y en una ruta, así que lo que puede llevar no es cuestión
	// de gusto
	validos := []string{"base", "antes-del-upgrade", "v2.4.7", "con_guion_bajo"}
	invalidos := []string{"", ".oculto", "-empieza-por-guion", "con espacio", "con/barra", "con;punto-y-coma"}

	for _, nombre := range validos {
		if !ValidTemplateName(nombre) {
			t.Errorf("ValidTemplateName(%q) = false, want true", nombre)
		}
	}

	for _, nombre := range invalidos {
		if ValidTemplateName(nombre) {
			t.Errorf("ValidTemplateName(%q) = true, want false", nombre)
		}
	}
}

func TestElVolumenLlevaProyectoYNombre(t *testing.T) {
	// Dos proyectos con una plantilla llamada igual son dos plantillas
	if TemplateVolume("tienda", "base") == TemplateVolume("otra", "base") {
		t.Fatal("two projects derived the same template volume, want one each")
	}
}

func TestElTamanoSeLeeDeUnVistazo(t *testing.T) {
	casos := map[int64]string{
		0:          "0B",
		512:        "512B",
		1024:       "1.0KB",
		1536:       "1.5KB",
		155521024:  "148.3MB",
		1073741824: "1.0GB",
	}

	for bytes, esperado := range casos {
		if HumanSize(bytes) != esperado {
			t.Errorf("HumanSize(%d) = %q, want %q", bytes, HumanSize(bytes), esperado)
		}
	}
}
