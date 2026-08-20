## Why

Tres defectos de la capa de interacción, encontrados auditando el estado actual
([terminal-ux.md](../../../docs/research/terminal-ux.md)). Son pequeños, independientes y
se notan cada día:

1. **`hm transfer-db` pide la contraseña de la base de datos en claro.** Usa `read -p` sin
   `-s`, así que la contraseña de un entorno de cliente aparece en pantalla y **queda en el
   scrollback del terminal**. Las guías de diseño de CLI lo prohíben explícitamente.
2. **No se respeta `NO_COLOR`.** Tampoco `TERM=dumb`, y no existe `--no-color`. Es el
   estándar que respetan Docker, git, ripgrep y prácticamente todo lo moderno; ignorarlo
   rompe a quien usa un terminal sin color, un lector de pantalla o un pipeline que no
   interpreta ANSI.
3. **Cada pregunta borra la pantalla.** `custom_question` y `custom_select` llaman a
   `clear`, así que en `hm setup` cada respuesta borra lo que el usuario acaba de leer,
   incluida la pregunta anterior y su respuesta.

Backlog: **UX-01**, **UX-02**, **UX-03**.

## What Changes

- La contraseña de `transfer-db` se lee sin eco, con salto de línea explícito después.
  Se revisa que no queden otras preguntas de credenciales sin proteger.
- Un único punto de decisión sobre el color en `load_colors`, que atiende, por orden:
  `--no-color`, `NO_COLOR`, `TERM=dumb`, `FORCE_COLOR`/`CLICOLOR_FORCE` y, por último, si
  stdout es un TTY. Cuando el color está desactivado, las variables de color quedan vacías
  y el resto del código no se entera.
- `--no-color` se suma a los flags globales (`--json`, `--no-json`, `--yes`, `--force`).
- Se elimina el borrado de pantalla del flujo de preguntas. El caso donde borrar sí tiene
  sentido —un TUI— se resolverá con el buffer de pantalla alternativo, que es reversible.

## Non-goals

- No se rediseña la ayuda (UX-04), ni se añade progreso (UX-05), ni el selector navegable
  (UX-06), ni la biblioteca de componentes (UX-07). Son cambios propios.
- No se cambia ningún texto ni ningún contrato de salida: el JSON, los códigos de salida y
  el modo no interactivo se quedan como están.
- No se añaden dependencias.

## Capabilities

### New Capabilities
- `terminal-presentation`: cómo decide Dockergento si colorear su salida y cómo se comporta
  el flujo de preguntas respecto al contenido que ya hay en pantalla.

### Modified Capabilities
<!-- Ninguna. `cli-output-contract` sigue igual: esto es el "cómo se ve", no el "qué se dice". -->

## Impact

- **Código**: `console/tasks/load_properties.sh` (`load_colors`),
  `console/helpers/global_options.sh` (`--no-color`),
  `console/components/input_info.sh` (borrado de pantalla y lectura sin eco),
  `console/commands/transfer-db.sh` (la contraseña),
  `console/tasks/version_manager.sh` (el bucle de edición de versiones).
- **Datos**: `data/command_descriptions.json` para documentar `--no-color`.
- **Documentación**: `README.md` (variables de entorno que se respetan).
- **Proyectos existentes**: sin migración.
- **Dependencias**: ninguna.
