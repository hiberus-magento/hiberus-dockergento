## Why

Ejecutar `hm start` o `hm rebuild` desde un git worktree **secuestra el entorno del
checkout principal**. Está comprobado en laboratorio: Docker Compose identifica un proyecto
por su *nombre*, no por su ruta, y como el `COMPOSE_PROJECT_NAME` está versionado en
`config/docker/properties.json`, el worktree hereda el mismo nombre. El resultado es que
Compose no crea un segundo entorno: **recrea los contenedores existentes con los bind
mounts del directorio desde el que se invoca**.

```
up desde el checkout principal → mount .../commerce/app
up desde el worktree           → "Container recreated", mount .../wt-a/app
```

A partir de ahí, el entorno "principal" está sirviendo el código del worktree sin que nadie
lo haya pedido, y un `hm down -v` desde el worktree destruye los volúmenes del proyecto,
base de datos incluida.

Esto ya bloquea el trabajo con varios agentes en paralelo y es la única entrada urgente del
backlog: no depende de ninguna otra y evita una pérdida de datos real.

Backlog: **WT-01**.

## What Changes

- Detección de worktree: `hm` averigua si el directorio actual es un worktree y cuál es su
  checkout principal, usando `git rev-parse --path-format=absolute --git-common-dir`.
- Resolución de rutas contra el checkout principal: desde un worktree, los ficheros de
  Compose y las propiedades se resuelven en rutas absolutas del checkout principal, y se
  invoca a Compose con `--project-directory` apuntando allí. Así los comandos de lectura y
  de ejecución (`bash`, `exec`, `magento`, `mysql`, `logs`) siguen funcionando y atacan el
  entorno correcto.
- Guardarraíles: los comandos que alteran la topología del entorno (`start`, `stop`,
  `restart`, `rebuild`, `down`, `setup`, `install`, `docker-stop-all`) se **bloquean** al
  ejecutarse desde un worktree, con un mensaje que explica el motivo y ofrece la salida.
- `--force` permite ejecutarlos igualmente, de forma consciente.
- Nuevo código de salida para "operación bloqueada por seguridad en un worktree".
- Variable de escape `HM_PROJECT_DIR` para casos en que la detección automática no sirva.

## Non-goals

- **No** se crean entornos por worktree: eso es WT-02 y depende del proxy global y de los
  snapshots. Aquí un worktree comparte el entorno del checkout principal.
- No se resuelve que el código montado en los contenedores sea el del worktree: sigue
  siendo el del checkout principal, y el mensaje de bloqueo debe dejarlo claro para no
  crear una expectativa falsa.
- No se cambia el `COMPOSE_PROJECT_NAME` ni el formato de `properties.json`.
- No se añade el comando `hm worktree`.

## Capabilities

### New Capabilities
- `worktree-safety`: cómo se comporta la CLI cuando se ejecuta desde un git worktree del
  proyecto, qué resuelve contra el checkout principal y qué operaciones impide.

### Modified Capabilities
<!-- Ninguna. -->

## Impact

- **Código**: nuevo `console/helpers/worktree.sh`; `bin/run` (resolución de rutas y
  validación de comandos); `console/helpers/process_hm_options.sh` (`--force`);
  `console/commands/docker-stop-all.sh`.
- **Documentación**: `README.md` y una nota en `docs/` sobre el trabajo con worktrees.
- **Proyectos existentes**: no requiere migración. Quien no use worktrees no nota ningún
  cambio; quien los use pasa de un comportamiento destructivo silencioso a uno bloqueado y
  explicado.
- **Dependencias**: ninguna nueva. `git` ya es requisito del proyecto.
