## Why

`hm --help` —lo primero que ejecuta cualquiera que se acerca a la herramienta, y lo que se
teclea cada vez que uno no recuerda un comando— tarda **entre 5,7 y 6,2 segundos** en una
máquina de desarrollo normal. No es percepción: es el tiempo medido, repetido tres veces.

La causa está localizada. Pintar el listado lanza **143 procesos `jq`**, uno por cada
consulta de cada comando (información, descripción y marca de plataforma, por los 45
comandos), cuando el fichero que se consulta es **el mismo** en las 143 llamadas y cabe
entero en memoria.

Además hay un coste fijo que paga *cada* invocación de `hm`, incluidas las que no lo
necesitan: detectar la versión de Docker Compose (188 ms), calcular la versión de Magento
leyendo un `composer.lock` de 1,6 MB (77 ms), preguntar la versión de `hm` a git (56 ms) y
validar la configuración de Compose (72 ms).

Backlog: **PERF-01** a **PERF-04**.

## Medidas de partida

| Comando | Tiempo | Procesos lanzados |
|---|---|---|
| `hm --help` | **5,7 – 6,2 s** | 143 `jq` + 3 `git` |
| `hm doctor --json` | 3,9 – 4,6 s | 12 procesos de check, secuenciales |
| `hm describe --json` | 1,2 – 1,6 s | 1 `docker ps`, 1 `compose config`, 1 `docker exec` |
| `hm list --json` | 1,2 – 1,4 s | 1 `docker ps` |
| `hm --version` | **0,30 s** | suelo de arranque |

Coste unitario medido: `jq` ≈ 36 ms por proceso · `docker compose version` 188 ms ·
`docker info` 78 ms · `compose config -q` 72 ms · `git rev-parse` 17 ms · `git describe`
56 ms · `jq` sobre `composer.lock` 77 ms.

## What Changes

- **Listado de comandos en una sola pasada**: una única invocación de `jq` produce toda la
  tabla de comandos y el bucle de presentación se hace en Bash. Objetivo: `hm --help` por
  debajo de **0,5 s**.
- **Coste fijo perezoso**: la detección de Docker Compose, la versión de `hm` y la versión
  de Magento se calculan **sólo cuando se necesitan**, no en cada invocación.
- **Validación de Compose con caché fuera del repositorio**: si los ficheros no han cambiado
  desde la última validación correcta, no se vuelve a invocar a Compose. La caché vive en
  `~/.hm/cache/`, no en el proyecto, para no tener que tocar el `.gitignore` de cada
  repositorio ni arriesgarse a versionarla.
- **`hm doctor` en paralelo**: las comprobaciones se lanzan concurrentemente en lugar de una
  detrás de otra. Objetivo: por debajo de **1,5 s**.
- **Presupuesto de rendimiento vigilado por un test**: una prueba que falla si `hm --help`
  o el arranque se pasan del presupuesto, para que esto no vuelva a degradarse sin que nadie
  se dé cuenta.

## Non-goals

- No se cambia ninguna salida ni ningún contrato: el JSON, los códigos de salida y el texto
  legible se mantienen idénticos.
- No se reescribe nada en otro lenguaje. Sigue siendo Bash (decisión D del proyecto).
- No se introduce un demonio ni un servicio residente que "precaliente" nada.
- No se toca el rendimiento *dentro* de los contenedores (Magento, Composer, montajes en
  macOS): eso es ENV-08 y es otra investigación.

## Capabilities

### New Capabilities
- `cli-performance`: qué coste puede tener la CLI en arrancar y en responder, y qué trabajo
  tiene derecho a hacer en cada invocación.

### Modified Capabilities
<!-- Ninguna: las capacidades existentes conservan su comportamiento observable. -->

## Impact

- **Código**: `console/helpers/print_help.sh` (el bucle de 143 `jq`), `bin/run`
  (coste fijo y orden de arranque), `console/helpers/version.sh` (detección de Compose),
  `console/tasks/set_environment_labels.sh` (cálculo perezoso),
  `console/tasks/validate_docker_compose.sh` (caché), `console/commands/doctor.sh`
  (paralelismo).
- **Caché en disco**: se introduce `~/.hm/cache/`, fuera de cualquier repositorio. No
  requiere cambios en los proyectos.
- **Proyectos existentes**: sin migración. El comportamiento observable no cambia.
- **Dependencias**: ninguna nueva.
