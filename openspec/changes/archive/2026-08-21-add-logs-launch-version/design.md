## Context

Los tres comandos son pequeños, pero cada uno toca una regla ya establecida y conviene decidir
cómo la respeta antes de escribirlos.

`hm logs` produce **datos**: su salida es la de otro proceso y puede ser infinita con `-f`.
Eso lo coloca en la misma familia que `mysqldump` o `exec`, no en la de `describe`.

`hm launch` no produce salida: produce un **efecto** fuera del terminal. Es el primer comando de
la CLI que hace algo en el escritorio del usuario, y hay que decidir qué significa eso cuando no
hay escritorio.

`hm version` ya existe a medias como `--version`, y hay que decidir qué comparten.

## Goals / Non-Goals

**Goals**
- Cubrir la operación más frecuente cuando algo falla (`logs`) sin exigir saber que hay Compose
  debajo.
- Que el panel deje de rodear a la CLI: sus acciones tienen que ser comandos.
- Un `version` que sirva para pegar en un informe de error.

**Non-Goals**
- No se filtra ni se colorea el registro: eso es trabajo de `grep` y de quien lo lea.
- No se agrega ni se persiste nada: no somos un recolector de logs.
- `hm launch` no arranca el entorno si está parado. Abrir el navegador contra un entorno caído
  no es lo que se ha pedido, y arrancarlo por sorpresa tampoco.

## Decisions

### 1. `logs` es un comando transparente

Entra en la lista de comandos cuya salida son datos, con dos consecuencias buscadas: nunca se
envuelve en JSON, y las opciones globales sólo se interpretan **antes** del nombre del comando,
de modo que `-f`, `--tail` y `--since` llegan intactas a Compose. Sin eso, `hm logs -f` sería un
accidente esperando a que alguien añadiera `-f` como abreviatura de `--force`.

### 2. Un servicio que no existe se rechaza aquí, no en Compose

`docker compose logs inventado` responde con un error propio que menciona rutas de ficheros
YAML. La CLI ya sabe qué servicios tiene el proyecto, así que valida el nombre y falla con el
código de servicio y con la lista de los disponibles. Es la diferencia entre un error que
enseña y uno que hay que descifrar.

El coste es una llamada a `docker compose config --services`, que sólo se paga cuando se nombra
un servicio: `hm logs` a secas no valida nada porque no hay nada que validar.

### 3. `launch` abre si hay dónde abrir, y si no, dice la dirección

Tres situaciones y una sola regla —hacer lo útil en cada contexto—:

| Contexto | Comportamiento |
|---|---|
| Terminal, con `open`/`xdg-open` | Abre el navegador y dice qué abrió |
| `--json`, o salida redirigida | Escribe la URL y no abre nada |
| Terminal sin lanzador (Linux sin escritorio) | Escribe la URL y lo explica |

Así `hm launch --json | jq -r .data.url` es utilizable en un script, y `hm launch` en una sesión
SSH no falla: informa.

La URL sale de la misma fuente que `hm describe`, no de una construcción propia con el dominio:
una sola definición de cuál es la dirección del proyecto.

### 4. Un destino por invocación, con la tienda por defecto

`--admin`, `--mailhog`, `--rabbitmq` y `--search` seleccionan destino; sin opción, la tienda.
Nombrar dos es un error de uso, no una lista de pestañas que abrir: abrir cuatro pestañas por un
`hm launch --admin --search` mal escrito es peor que negarse.

Los nombres son los de las claves que ya publica `describe`, para que no haya que traducir entre
lo que se lee y lo que se escribe.

### 5. `version` y `--version` comparten la fuente, no la salida

La referencia exacta de la CLI ya la calcula `hm_version_data`. El comando la reutiliza y añade
lo que `--version` no tiene: las versiones de Docker y de Compose, que es el motivo de que el
comando exista.

`--version` no cambia. Es lo que ya consumen los scripts, y ampliarlo significaría pagar dos
llamadas a Docker en el camino más corto de la herramienta —el que tiene un presupuesto de
rendimiento vigilado por un test.

`version` no exige proyecto: sirve para informar de un problema, y el problema puede ser
precisamente que no hay proyecto.

### 6. El panel llama a los comandos, no los reimplementa

`o` pasa de resolver la URL con `jq` y buscar un lanzador a ejecutar `hm launch`, y `l` de
`hm docker-compose logs -f --tail 100` a `hm logs -f --tail 100`. El panel pierde código y gana
la propiedad que ya tenía por diseño: lo que la CLI decide vale también dentro del panel.

## Risks / Trade-offs

- **`logs -f` no termina.** Es lo que se pide al escribirlo, y dentro del panel se ejecuta con el
  terminal cedido, así que `Ctrl-C` corta el seguimiento y devuelve al panel.
- **Validar el servicio cuesta una llamada a Compose.** Sólo cuando se nombra uno, y el
  alternativa es un error ilegible.
- **`launch` depende del lanzador del sistema.** Sin él escribe la URL, que es exactamente lo
  que se necesita para copiarla.
- **`version` llama a Docker dos veces**, y por eso es un comando y no una ampliación de
  `--version`.

## Migration Plan

Nada que migrar: tres comandos nuevos y dos llamadas del panel que pasan a apuntar a ellos.
`hm docker-compose logs` sigue funcionando para quien lo tenga en un alias.

## Open Questions

Ninguna.
