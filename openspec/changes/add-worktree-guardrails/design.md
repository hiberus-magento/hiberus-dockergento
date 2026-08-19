## Context

`bin/run` resuelve hoy todo contra `$PWD`:

```bash
export CUSTOM_PROPERTIES_DIR="$PWD/$DOCKER_CONFIG_DIR"
export DOCKER_COMPOSE="$compose_cmd -f $DOCKER_COMPOSE_FILE -f $DOCKER_COMPOSE_FILE_MACHINE"
```

con `DOCKER_COMPOSE_FILE` en ruta relativa. En un proyecto real, `docker-compose.yml`,
los overlays y `config/docker/properties.json` **están versionados en git**, así que un
worktree los hereda y `hm` arranca sin quejarse, operando sobre el proyecto del checkout
principal pero con los binds del worktree.

Datos comprobados en el entorno del equipo:
- `docker compose -p <nombre> ps|exec` funciona desde cualquier directorio, **sin** ficheros
  de Compose: Compose resuelve por nombre de proyecto. Esto es lo que hace viable el modo
  "compartir entorno".
- `git rev-parse --path-format=absolute --git-common-dir` devuelve, desde un worktree, la
  ruta absoluta del `.git` del checkout principal; su directorio padre es el checkout.
- En un worktree, `.git` es un **fichero** (`gitdir: …/.git/worktrees/<nombre>`), no un
  directorio. Importa para cualquier montaje futuro de `.git`, y por eso se documenta aquí.

## Goals / Non-Goals

**Goals:**
- Que sea imposible destruir o secuestrar el entorno principal sin quererlo.
- Que los comandos de lectura y ejecución sigan funcionando desde un worktree.
- Que el mensaje de bloqueo enseñe qué está pasando, no sólo que se ha impedido algo.

**Non-Goals:**
- Entornos por worktree.
- Que el contenedor sirva el código del worktree.

## Decisions

### 1. Detección con git, no con heurísticas de ruta

`git rev-parse --path-format=absolute --git-common-dir` es la fuente de verdad: si su
resultado, quitando el `/.git` final, es distinto del `git rev-parse --show-toplevel`
actual, estamos en un worktree y ese resultado es el checkout principal.

Alternativas descartadas: comparar rutas por convención de nombres (frágil) o leer el
fichero `.git` a mano (reimplementa git). Requiere git ≥ 2.31 por `--path-format`; si no
está disponible, se cae a `--git-common-dir` relativo y se resuelve con `cd`.

### 2. Resolver contra el checkout principal y añadir `--project-directory`

Desde un worktree, `hm` construye `DOCKER_COMPOSE` con rutas **absolutas** a los ficheros
del checkout principal y añade `--project-directory <checkout principal>`, para que los
binds relativos del fichero se resuelvan allí y no en el worktree. Ésta es la línea exacta
que hoy provoca el secuestro.

Alternativa descartada: invocar sólo con `-p <nombre>` y sin ficheros. Funciona para
`exec`/`ps` pero no para nada que necesite la definición de servicios, y deja el
comportamiento dependiendo de qué subcomando se use.

### 3. Bloquear por comando, no por operación de Docker

La lista de comandos bloqueados es explícita y vive junto a la lista de comandos que hoy
saltan la validación de Docker en `bin/run`. Es fácil de leer y de auditar, frente a
intentar interceptar verbos de Compose.

Bloqueados: `start`, `stop`, `restart`, `rebuild`, `down`, `setup`, `install`,
`create-project`, `docker-stop-all`. Permitidos: todo lo demás, incluidos `bash`, `exec`,
`magento`, `composer`, `mysql`, `mysqldump`, `describe` y `list`.

`composer` merece una nota: en mac copia y sincroniza el vendor entre host y contenedor, así
que desde un worktree sincronizaría contra el checkout principal. No se bloquea —no es
destructivo para el entorno— pero se emite un aviso.

### 4. El mensaje de bloqueo explica, no sólo prohíbe

Debe decir tres cosas: que se está en un worktree y cuál es el checkout principal; qué
habría pasado (recrear los contenedores del entorno principal con los montajes de este
worktree); y qué hacer — ir al checkout principal, o usar `--force` si de verdad es lo que
se quiere. Sin esto, el guardarraíl se percibe como una molestia y se pasa a `--force` por
costumbre.

### 5. `--force` explícito y no memorizable

`--force` afecta a una sola invocación. No hay variable de entorno ni fichero de
configuración que lo desactive de forma permanente, precisamente para que no acabe exportado
en un `.zshrc` y el guardarraíl deje de existir.

### 6. `HM_PROJECT_DIR` como escape

Si está definida, se usa como checkout principal y se salta la detección. Sirve para
entornos donde git no esté disponible o para pruebas, y es lo que permitirá a WT-02 apuntar
a otro sitio sin reescribir la detección.

### 7. Código de salida propio

Se reserva el `6` para "operación bloqueada por seguridad", dentro del rango libre que dejó
el contrato de salida. Permite a un agente distinguir "esto está prohibido aquí" de "esto ha
fallado".

### 8. mac y linux

Sin diferencias en la detección ni en el bloqueo. La única nota específica es la del
`composer` en mac de la decisión 3.

## Risks / Trade-offs

- **Falso positivo: un proyecto que vive dentro de un worktree por diseño** → `--force` y
  `HM_PROJECT_DIR` cubren el caso; además el bloqueo sólo aplica si el checkout principal
  contiene un proyecto Dockergento.
- **Costumbre de usar `--force`** → se mitiga con el mensaje de la decisión 4 y con que
  `--force` no sea persistente; conviene revisar en unas semanas si se está abusando.
- **Coste de invocar git en cada ejecución** → una llamada a `git rev-parse`, del orden de
  milisegundos, y sólo cuando el directorio está bajo control de git.
- **Expectativa falsa**: alguien puede creer que, al no bloquearse `hm magento`, está
  ejecutando su código del worktree. No es así. Se documenta y se avisa en el mensaje.

## Migration Plan

Se publica como una versión normal. No hay estado que migrar. El changelog debe explicar el
comportamiento destructivo anterior, porque puede que alguien lo haya sufrido sin saber por
qué su entorno "se volvió loco".

Reversión: revertir el commit; no queda estado persistente.

## Open Questions

- ¿Debería `hm` avisar también cuando se detecta que el entorno **ya** fue secuestrado, es
  decir, cuando los montajes de los contenedores no coinciden con el checkout principal?
  Es detectable comparando con `docker inspect`. Propuesta: sí, como comprobación de
  `hm doctor` en su cambio, no aquí.
