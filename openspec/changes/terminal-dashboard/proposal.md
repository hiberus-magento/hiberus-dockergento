## Why

La máquina de un desarrollador del departamento tiene hoy **nueve entornos Dockergento**, y
la única forma de saber qué hay levantado, en qué rama está cada uno y cuál está comiéndose
los puertos es ejecutar comandos uno a uno y cruzar la salida a mano. `hm list` y
`hm doctor` responden a esas preguntas, pero cada una por separado y sin poder actuar.

Ese es el hueco que cierra un panel de terminal: **ver la flota y operarla desde el mismo
sitio**, sin salir del terminal ni recordar en qué directorio estaba cada proyecto.

Es además el momento adecuado. Los tres comandos que hacen falta ya existen y ya hablan
JSON, así que el panel no necesita inventarse nada: es el tercer consumidor del contrato que
se diseñó precisamente para esto.

Backlog: **TUI-01**. Depende de **UX-07** (la biblioteca de componentes de terminal).

## What Changes

- Nuevo comando `hm tui`: un panel a pantalla completa con la flota de entornos de la
  máquina, su estado, su rama y los avisos del diagnóstico.
- Navegación con teclado: moverse por la lista, abrir el detalle de un entorno, y desde ahí
  ejecutar las acciones frecuentes —arrancar, parar, reiniciar, ver logs, abrir en el
  navegador— sin escribir comandos.
- Los datos salen de `hm list --json`, `hm describe --json` y `hm doctor --json`. El panel
  **no** lee Docker por su cuenta ni duplica lógica: presenta.
- Las acciones invocan los comandos existentes, así que lo que se puede hacer desde el panel
  es exactamente lo que se puede hacer desde la CLI, con las mismas protecciones.
- Al salir, el terminal queda como estaba: el panel vive en la pantalla alternativa.

## Non-goals

- **No entra nada que no exista ya.** Los snapshots de base de datos (DB-01) y los entornos
  por worktree (WT-02) tendrán su sitio en el panel cuando existan, no antes.
- No se cambia lo que hace `hm` sin argumentos: hoy muestra la ayuda y así se queda (ver la
  decisión sobre el punto de entrada en el diseño).
- No se incluyen acciones destructivas: nada de `down -v` ni de borrar volúmenes desde el
  panel en esta iteración.
- No es un gestor de Docker: si algo no habla de proyectos, entornos o worktrees, no entra.
- No hay dependencias nuevas. Ni `fzf`, ni `gum`, ni `ncurses`.

## Capabilities

### New Capabilities
- `fleet-dashboard`: qué muestra el panel de terminal, cómo se navega y qué se puede hacer
  desde él.

### Modified Capabilities
<!-- Ninguna. Consume `environment-introspection`, `environment-diagnostics` y
     `terminal-control` sin cambiar sus requisitos. -->

## Impact

- **Código**: nuevo `console/commands/tui.sh` y, si el bucle crece, tareas de apoyo en
  `console/tasks/tui/`. Consume `console/components/tui.sh` (UX-07).
- **Datos**: `data/command_descriptions.json`, en el grupo de entorno.
- **Documentación**: `docs/tui.md` con las teclas y lo que hace cada una.
- **Rendimiento**: `hm list --json` tarda ~890 ms y `hm doctor --json` ~1,3 s. El panel debe
  pintar algo antes de tener los datos (ver el diseño), y hay un presupuesto vigente que no
  puede empeorar.
- **Proyectos existentes**: sin impacto. El panel no modifica nada por su cuenta.
- **Dependencias**: ninguna.
