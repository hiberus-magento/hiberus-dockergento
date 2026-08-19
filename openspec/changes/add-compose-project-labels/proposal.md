## Why

Hoy no hay forma de preguntarle a Docker "qué entornos Dockergento hay en esta máquina".
Los contenedores sólo llevan las etiquetas estándar de Compose
(`com.docker.compose.project`), que dicen el nombre del proyecto pero no si ese proyecto es
un checkout principal o un worktree, con qué perfil se levantó, ni sobre qué versión de
Magento corre. Cualquier herramienta que quiera enumerar la flota tiene que adivinar a
partir de nombres de contenedor, que es exactamente el patrón que ya nos ha dado problemas:
`docker ps -f name=db` casa por subcadena contra **todos** los proyectos de la máquina.

Estas etiquetas son el mecanismo de descubrimiento del que dependen `hm list`, `hm clean`,
el TUI, el futuro dashboard y la detección de worktrees huérfanos. Se hacen ahora porque
son baratas y porque, sin ellas, todo lo demás necesita un fichero de registro paralelo que
se desincroniza.

Backlog: **ENV-02**.

## What Changes

- La plantilla `docker-compose.template.yml` incorpora un bloque de etiquetas `hm.*` en
  todos los servicios, con interpolación de variables de entorno para que no haya que
  regenerar los ficheros cuando cambian los valores.
- `bin/run` exporta las variables que alimentan esas etiquetas antes de invocar a
  `docker compose`.
- Etiquetas definidas: `hm.project`, `hm.root`, `hm.worktree`, `hm.profile`,
  `hm.magento`, `hm.agent` y `hm.version`.
- `console/helpers/docker.sh` gana una función de descubrimiento por etiquetas, con
  retroceso a las etiquetas estándar de Compose para entornos creados antes de este cambio.

## Non-goals

- No se implementa `hm list` ni ningún comando nuevo: aquí sólo se estampan y se leen las
  etiquetas.
- No se estampa la rama de git (ver la decisión sobre datos volátiles en el diseño).
- No se crea ningún fichero de registro ni base de datos local de proyectos.
- No se cambian los nombres de contenedor ni el `COMPOSE_PROJECT_NAME`.

## Capabilities

### New Capabilities
- `environment-discovery`: cómo se identifica y se descubre un entorno Dockergento en la
  máquina a partir de metadatos estampados en sus contenedores.

### Modified Capabilities
<!-- Ninguna. -->

## Impact

- **Código**: `docker-compose/docker-compose.template.yml`, `bin/run`,
  `console/helpers/docker.sh`, `console/tasks/write_from_docker-compose_templates.sh`.
- **Proyectos existentes**: **sí requiere migración**, pero es benigna. Los contenedores ya
  creados no tienen las etiquetas nuevas y sólo las obtendrán al regenerar el
  `docker-compose.yml` (`hm setup -f`) y recrear los contenedores. Mientras tanto, el
  descubrimiento retrocede a las etiquetas estándar de Compose y esos proyectos aparecen
  como entornos "sin metadatos" en lugar de desaparecer.
- **Dependencias**: ninguna nueva.
