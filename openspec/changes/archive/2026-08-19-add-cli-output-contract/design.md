## Context

`hm` es 100 % Bash y toda su salida pasa hoy por `console/components/print_message.sh`,
que escribe texto con códigos ANSI de color en stdout. La entrada interactiva vive en
`console/components/input_info.sh` (`read -rp`) y en los asistentes `ai-*`. El router
`bin/run` ya intercepta opciones globales (`--help`, `-h`) en
`console/helpers/process_hm_options.sh`, así que existe un punto natural donde añadir
flags globales.

Los consumidores no humanos de la CLI ya son reales: CI, y sobre todo agentes de IA que
ejecutan `hm` dentro de proyectos Magento. El caso de `hm mysql -q` inalcanzable desde
un agente (corregido en la 1.4.5) es la prueba de que el problema no es teórico.

Restricción de fondo: **no añadir dependencias**. `jq` ya es requisito del proyecto y
basta para construir y escapar JSON correctamente.

## Goals / Non-Goals

**Goals:**
- Que cualquier comando de lectura se pueda consumir de forma programática sin parsear
  texto con colores.
- Que ningún comando se quede bloqueado esperando entrada cuando no hay nadie que escriba.
- Que el motivo de un fallo se pueda distinguir por código de salida, sin leer el mensaje.
- Que la experiencia interactiva no cambie en absoluto.

**Non-Goals:**
- Definir el contenido de `describe`/`list`: aquí sólo se fija la envoltura.
- Convertir a JSON la salida de los comandos que emiten **datos** (ver decisión 2).
- Internacionalizar los mensajes.

## Decisions

### 1. Detección de formato: TTY primero, flag después

`--json` fuerza JSON, `--no-json` fuerza texto y, sin flags, se usa `[ -t 1 ]` sobre
**stdout**: TTY → texto, no TTY → JSON. Es la convención que se ha consolidado en las CLIs
pensadas para agentes y evita tener que recordar el flag en cada invocación.

Alternativa descartada: JSON sólo con flag explícito. Más conservadora, pero deja el
problema intacto para quien canaliza la salida sin saber que existe el flag.

Se decide sobre **stdout** y no sobre stdin porque el consumidor del formato es quien lee
la salida. La detección de stdin sigue usándose sólo donde tiene sentido semántico (por
ejemplo `hm mysql < dump.sql`).

### 2. Clasificación de comandos: informativos, de flujo y de paso

No todos los comandos pueden emitir JSON, y forzarlo rompería usos legítimos:

| Clase | Ejemplos | Comportamiento |
|---|---|---|
| **Informativos** | `describe`, `list`, `doctor`, `compatibility`, `version` | Respetan `--json` y la detección de TTY |
| **De flujo de datos** | `mysqldump`, `mysql -q`, `logs`, `copy-from-container` | **Nunca** se envuelven: su stdout es el dato. Los mensajes de progreso van a stderr |
| **De paso (passthrough)** | `exec`, `bash`, `magento`, `composer`, `npm`, `n98-magerun` | Salida transparente del proceso hijo. Sólo se normalizan los errores **del propio `hm`** |

Esta clasificación es la parte del diseño con más riesgo de romper flujos existentes:
`hm mysqldump fichero.sql` debe seguir escribiendo SQL, no un objeto JSON.

### 3. Envoltura JSON

Éxito:

```json
{ "schema_version": 1, "command": "describe", "ok": true, "data": { } }
```

Error (por **stderr**, y siempre con código de salida distinto de cero):

```json
{ "schema_version": 1, "command": "start", "ok": false,
  "error": { "code": 3, "type": "docker_unavailable",
             "message": "Docker daemon is not running",
             "hint": "Start Docker Desktop and retry" } }
```

`schema_version` desde el primer día: es lo que permitirá evolucionar el contrato sin
romper al TUI, al dashboard ni a los agentes.

### 4. Códigos de salida

`0` correcto · `1` error genérico · `2` argumentos inválidos · `3` Docker no disponible ·
`4` proyecto no configurado · `5` servicio no levantado.

Se dejan libres del 6 al 9 para futuros cambios (worktrees usará uno). Se evita el rango
≥ 64 de `sysexits.h` por simplicidad y porque nadie lo espera en una CLI de este tipo.

### 5. Modo no interactivo

`--yes` y `HM_NON_INTERACTIVE=1` son equivalentes; la variable existe porque un agente o
un job de CI puede exportarla una vez en lugar de recordarla en cada llamada. Ya existe
`USE_DEFAULT_SETTINGS`, pero sólo cubre parte del flujo de `setup`/`install`: se mantiene
por compatibilidad y pasa a ser un caso particular del nuevo modo.

Regla: en modo no interactivo, toda pregunta usa su valor por defecto; si una pregunta no
tiene valor por defecto razonable, se falla con código 2 y un mensaje que diga **qué flag
hay que pasar** para no tener que preguntar.

Alternativa descartada: responder siempre "sí" a todo. Peligroso en comandos destructivos.
`--yes` significa "no me preguntes lo que ya puedes deducir", no "haz lo que sea".

### 6. Implementación en Bash

Un componente nuevo, `console/components/print_json.sh`, con `json_success`,
`json_error` y `json_kv`, construyendo siempre con `jq -n --arg` para que el escapado sea
correcto. `print_message.sh` no cambia de firma: se le añade la capacidad de callar la
salida decorativa cuando el modo es JSON, para que los `print_processing` no contaminen
stdout.

## Risks / Trade-offs

- **Romper scripts internos del equipo que dependen de la salida de texto** → la salida de
  texto no cambia mientras haya TTY; el cambio sólo aflora al canalizar. Se anuncia en el
  changelog y se ofrece `--no-json`.
- **`hm mysqldump` o `hm logs` envueltos en JSON por error** → la clasificación de la
  decisión 2 se materializa en una lista explícita en el código, y se cubre con una
  comprobación manual en las tareas.
- **Códigos de salida distintos rompen comprobaciones `$? -eq 1`** → documentado como
  cambio menor con **BREAKING** en la proposal; comprobar "distinto de cero" sigue valiendo.
- **Un `read` olvidado deja colgado a un agente** → la auditoría es una tarea explícita, y
  se añade una comprobación que ejecuta cada comando con stdin cerrado y sin TTY.
- **Divergencia mac/linux**: ninguna. `[ -t 1 ]`, `read` y `jq` se comportan igual en
  ambas; lo único específico de plataforma en el repositorio (`sed -i`, rutas de montaje)
  no interviene aquí.

## Migration Plan

No hay migración de datos ni de configuración de proyectos. Se despliega como una versión
normal de `hm` (`hm update`). El anuncio en el changelog debe incluir la tabla de códigos
de salida y la advertencia sobre canalizar salida.

Reversión: revertir el commit; ningún estado persistente queda tocado.

## Open Questions

- ¿`--yes` debe implicar también aceptar acciones destructivas (`down -v`) o hará falta un
  `--force` aparte? Propuesta: `--force` aparte, decidido en el cambio de ciclo de vida
  seguro (DB-03).
- ¿Se emite `schema_version` también en la salida de texto (por ejemplo en `hm version`)?
  Propuesta: sólo en JSON.
