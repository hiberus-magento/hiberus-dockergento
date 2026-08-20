## Why

El departamento va a empezar a probar versiones candidatas de `hm` sobre proyectos reales,
y a compartirlas entre compañeros para validarlas antes de publicar. Ese flujo necesita tres
cosas que hoy no existen, y una de ellas es una trampa activa:

1. **Saber qué versión estás ejecutando.** `hm --version` usa
   `git describe --tags --abbrev=0`, así que con once commits por encima del último tag
   seguía diciendo `1.4.5`. Quien reporta un fallo no puede decir sobre qué lo reporta.
2. **Cambiar de versión y volver.** No hay comando: hay que ir al directorio de instalación
   y hacer `git checkout` a mano, sabiendo cuál es ese directorio.
3. **Que actualizar no te saque de la versión que estás probando.** Al ponerse en un tag el
   checkout queda desacoplado (`detached HEAD`), y `hm update` ejecuta
   `git pull origin HEAD`, que **no falla**: trae el contenido de la rama por defecto del
   remoto. El compañero que estaba validando `1.5.0-rc.1` deja de estarlo, sin ningún aviso.
   Comprobado en un clon de prueba.

Backlog: **REL-01**, **REL-02**, **REL-03**.

## What Changes

- `hm --version` informa de la referencia exacta: la versión, los commits por encima del
  último tag, el sha corto, la rama y si el checkout tiene cambios sin guardar. Respeta el
  contrato de salida, así que admite `--json`.
- Nuevo `hm switch <versión|rama>`: cambia la instalación a esa referencia, regenera el
  autocompletado y dice a qué se ha cambiado. Con `hm switch --list` para ver qué hay
  disponible y `hm switch --stable` para volver a la rama estable.
- `hm switch` **se niega** a actuar si el directorio de instalación tiene cambios sin
  guardar, en lugar de descartarlos.
- `hm update` **se niega** a actuar en un checkout desacoplado y remite a `hm switch`.

## Non-goals

- No se cambia el mecanismo de instalación ni se empaqueta la herramienta de otra forma
  (Homebrew, binario): sigue siendo un clon de git.
- No se gestionan varias versiones instaladas a la vez. Se cambia la que hay.
- No se automatiza la publicación de releases ni se toca el workflow de imágenes.
- No se decide la política de ramas: eso es una convención de equipo y va documentada, no
  impuesta por el código.

## Capabilities

### New Capabilities
- `version-management`: cómo se sabe qué versión de la herramienta está instalada y cómo se
  cambia de una a otra sin perder trabajo ni salirse por accidente.

### Modified Capabilities
<!-- Ninguna. `cli-output-contract` se aplica al nuevo comando, sin cambiar sus requisitos. -->

## Impact

- **Código**: `console/helpers/process_hm_options.sh` (`--version`), nuevo
  `console/commands/switch.sh`, `console/commands/update.sh`.
- **Datos**: `data/command_descriptions.json`.
- **Documentación**: `README.md` con el modelo de ramas y tags y cómo probar una candidata;
  `docs/switch.md`.
- **Proyectos existentes**: sin impacto. Cambiar de versión de `hm` no toca los proyectos:
  las etiquetas del compose sólo entran al regenerar y las cachés viven en `~/.hm/`.
- **Dependencias**: ninguna. Sólo `git`, que ya es requisito.
