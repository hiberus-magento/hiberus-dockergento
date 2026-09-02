package core

import (
	"strings"
	"testing"
)

// Los errores que puede cometer este generador —un servicio escrito dos veces, un router
// apuntando a un servicio que se quitó— son todos invisibles hasta que algo está corriendo. Por
// eso se lee aquí en vez de levantando un entorno.

func overlay(perfil string, servicios []string, montajes []string) string {
	return OverlayFor("hm", "tienda-azul", perfil, "azul.tienda.test", "hm-proxy", servicios, montajes)
}

func TestCadaServicioSeEscribeUnaVez(t *testing.T) {
	// Una clave repetida en YAML no es una fusión: gana la última y el bloque anterior desaparece
	// sin decir nada
	servicios := []string{"phpfpm", "nginx", "db", "search", "redis", "varnish"}

	escrito := overlay("agent", servicios, nil)

	for _, servicio := range servicios {
		if veces := strings.Count(escrito, "\n  "+servicio+":"); veces > 1 {
			t.Errorf("%q aparece %d veces", servicio, veces)
		}
	}
}

func TestElPerfilQuitaLoQueNoLleva(t *testing.T) {
	escrito := overlay("agent", []string{"phpfpm", "nginx", "varnish", "rabbitmq"}, nil)

	if !strings.Contains(escrito, "varnish: !reset null") {
		t.Fatalf("el perfil de agente no lleva Varnish:\n%s", escrito)
	}

	if strings.Contains(escrito, "rabbitmq:\n    ports") {
		t.Fatal("ni la cola de mensajes")
	}
}

func TestNadiePublicaPuertos(t *testing.T) {
	// Sin esto cada rama publicaría los suyos, que es justo la colisión que el proxy vino a
	// terminar — con tantos entornos como ramas esta vez
	escrito := overlay("agent", []string{"phpfpm", "nginx"}, nil)

	if strings.Count(escrito, "ports: !reset []") != 2 {
		t.Fatalf("los dos servicios que quedan tienen que dejar de publicar:\n%s", escrito)
	}
}

func TestSoloElServicioWebLlevaElRouter(t *testing.T) {
	escrito := overlay("agent", []string{"phpfpm", "nginx"}, nil)

	if strings.Count(escrito, "traefik.http.routers.tienda-azul.rule") != 1 {
		t.Fatalf("un router y sólo uno:\n%s", escrito)
	}

	// Dos contenedores reclamando una regla es un router que responde con cualquiera de los dos
	if !strings.Contains(escrito, "Host(`azul.tienda.test`)") {
		t.Fatalf("y en la dirección de esta rama:\n%s", escrito)
	}
}

func TestUnPerfilSinWebNoAnunciaDireccion(t *testing.T) {
	// `lite` es código sin HTTP, y un router apuntando a un servicio que se quitó es un 404 con
	// una explicación que nadie tiene
	escrito := overlay("lite", []string{"phpfpm", "nginx"}, nil)

	if strings.Contains(escrito, "traefik.http.routers") {
		t.Fatalf("sin servicio web no hay router:\n%s", escrito)
	}
}

func TestConVarnishSeEntraPorVarnish(t *testing.T) {
	escrito := overlay("full", []string{"phpfpm", "nginx", "varnish"}, nil)

	if !strings.Contains(escrito, "loadbalancer.server.port: \"6081\"") {
		t.Fatalf("con todo el stack se entra por Varnish:\n%s", escrito)
	}
}

func TestLasDependenciasSeMontanSoloEnPhp(t *testing.T) {
	// Ahí es donde corre PHP, y donde `__DIR__` tiene que resolver dentro del worktree
	escrito := overlay("agent", []string{"phpfpm", "nginx"},
		[]string{"/code/tienda/vendor:/var/www/html/vendor:ro"})

	php := escrito[strings.Index(escrito, "  phpfpm:"):strings.Index(escrito, "  nginx:")]

	if !strings.Contains(php, "vendor:ro") {
		t.Fatalf("el montaje va en php:\n%s", escrito)
	}

	if strings.Count(escrito, "vendor:ro") != 1 {
		t.Fatalf("y sólo ahí:\n%s", escrito)
	}
}

func TestLaRedDelProxyEsExterna(t *testing.T) {
	escrito := overlay("agent", []string{"phpfpm", "nginx"}, nil)

	if !strings.Contains(escrito, "networks:\n  hm-proxy:\n    external: true") {
		t.Fatalf("la red del proxy la crea el proxy, no esto:\n%s", escrito)
	}
}

func TestEsYamlPlausible(t *testing.T) {
	// Sin el salto de línea de los montajes, la siguiente clave de servicio aterrizaba en la
	// misma línea y el documento dejaba de ser YAML
	escrito := overlay("agent", []string{"phpfpm", "nginx"},
		[]string{"/code/a:/var/www/html/vendor:ro", "/code/b:/var/www/html/node_modules:ro"})

	for _, linea := range strings.Split(escrito, "\n") {
		if strings.Count(linea, ":") > 0 && strings.Contains(linea, "- /code/a") &&
			strings.Contains(linea, "nginx") {
			t.Fatalf("dos cosas en una línea:\n%s", escrito)
		}
	}

	if !strings.Contains(escrito, "\n  nginx:\n") {
		t.Fatalf("nginx tiene que empezar en su propia línea:\n%s", escrito)
	}
}
