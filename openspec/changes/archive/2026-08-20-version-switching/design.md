## Context

`hm` se instala clonando el repositorio y enlazando `bin/run` en el `PATH`, así que la
instalación **es un checkout de git**. Eso es una ventaja para este cambio: cambiar de
versión es una operación de git sobre un directorio que ya conocemos (`$COMMAND_BIN_DIR`).

`hm update` ya hace `git pull` ahí, y ya distingue si la instalación es un clon de git o no.
La pieza que falta no es el mecanismo, es la ergonomía y las protecciones.

## Goals / Non-Goals

**Goals:**
- Que un compañero pueda decir con precisión qué versión está ejecutando.
- Que cambiar de versión y volver sea un comando, no un procedimiento.
- Que nadie se salga de la versión que está validando sin enterarse.
- Que nadie pierda cambios locales por cambiar de versión.

**Non-Goals:**
- Varias versiones instaladas en paralelo.
- Cambiar el modelo de distribución.

## Decisions

### 1. La versión se describe entera, no redondeada

`git describe --tags` sin `--abbrev=0`: en un tag da `1.5.0-rc.1`, y por encima de un tag da
`1.5.0-rc.1-7-gabc1234`. Esa segunda forma es justo la que hace falta cuando alguien prueba
una rama y encuentra un fallo.

Se añaden tres datos más: la rama (o "detached" con el tag al que apunta), si el checkout
tiene cambios sin guardar, y la ruta de instalación. Lo último porque en cuanto haya dos
instalaciones en una máquina, la pregunta "¿cuál estás usando?" aparece sola.

Alternativa descartada: fijar la versión en un fichero `VERSION`. Habría que acordarse de
subirlo y se desincroniza del tag; el tag ya es la fuente de verdad.

### 2. `switch` es git, con protecciones

`hm switch <ref>`:

1. Comprueba que la instalación es un clon de git; si no, error claro (como ya hace `update`).
2. Comprueba que **no hay cambios sin guardar**. Si los hay, se niega y los lista. Cambiar
   de versión no puede llevarse por delante el trabajo de nadie, ni siquiera con un stash
   automático: un stash silencioso es una forma elegante de perder cosas.
3. `git fetch --tags --prune`, porque sin eso no se ven las versiones nuevas.
4. `git checkout <ref>`.
5. Regenera el autocompletado, igual que `update`.
6. Informa de la versión resultante y de dónde leer qué cambió.

`--list` muestra las versiones (tags ordenados por versión) y las ramas remotas, marcando la
actual. `--stable` es azúcar para la rama estable.

### 3. `main` es la rama estable, y es una convención documentada

`--stable` va a `main`. No se hace configurable: si algún día cambia, se cambia una línea.
El modelo de ramas y tags se documenta en el README, pero **el código no lo impone** más allá
de ese atajo.

### 4. `update` se niega en un checkout desacoplado

Es la corrección más importante del cambio. Hoy `git pull origin HEAD` sobre un tag no falla:
trae la rama por defecto del remoto y te saca de la versión que estabas validando sin decir
nada. Con el cambio, `update` detecta el estado desacoplado y remite a `hm switch`, sin tocar
nada.

Ese es el fallo que hace inviable compartir candidatas: quien las prueba las pierde en su
primer `hm update`.

### 5. Cambiar de versión no toca los proyectos

Merece decirse explícitamente porque es la base de que volverse atrás sea seguro: las
etiquetas `hm.*` sólo entran en un proyecto al regenerar su configuración, las cachés viven
en `~/.hm/` y una versión antigua simplemente las ignora. Un `docker-compose.yml` generado
con 1.5.0 sigue siendo válido con 1.4.5, porque las variables interpoladas tienen valor por
defecto.

Se verifica de verdad, no se asume: hay una tarea que baja de versión con un proyecto real y
comprueba que sigue funcionando.

### 6. mac y linux

Sin diferencias. Todo es git y `readlink`, cuyo comportamiento difiere entre plataformas
pero que ya se resuelve en `bin/run` con `resolve_absolute_dir`.

## Risks / Trade-offs

- **Alguien cambia de versión con trabajo local a medias** → se niega y lo lista.
- **Quedarse en detached HEAD sin saberlo** → `--version` lo dice y `update` lo detecta.
- **Una versión candidata con un fallo grave** → volver es `hm switch --stable`, y por eso la
  seguridad de bajar de versión es una tarea de verificación explícita.
- **`switch` a una referencia que no existe** → error claro con `--list` como pista, y sin
  dejar el checkout a medias.

## Migration Plan

Ninguna. Comandos nuevos y una protección añadida.

Nota para el changelog: quien esté probando `1.5.0-rc.1` hoy debe saber que hasta que este
cambio esté publicado, `hm update` le saca del tag.

## Open Questions

- ¿Debe `hm switch` avisar cuando la versión de destino es anterior a la que generó la
  configuración de algún proyecto? Sería útil, pero requiere leer `hm.version` de los
  entornos y decidir qué hacer con el aviso. Propuesta: fuera de este cambio; anotado para
  cuando exista el TUI, que ya presenta esa información.
