## Context

`write_from_docker-compose_templates.sh` genera hoy `docker-compose.yml` sustituyendo
marcadores de la plantilla con `sed` en el momento del `setup`. Los ficheros resultantes
**están versionados en git** en los proyectos reales, lo que tiene dos consecuencias:
regenerarlos ensucia el diff de la rama en la que esté trabajando quien lo haga, y un
valor "horneado" en el fichero viaja a todos los worktrees de ese repositorio.

Al mismo tiempo, Docker Compose interpola variables de entorno (`${VAR:-default}`) en el
momento de `up`, sin necesidad de regenerar nada.

## Goals / Non-Goals

**Goals:**
- Que cualquier herramienta pueda enumerar los entornos de la máquina sin adivinar por
  nombres.
- Que la información estampada sea correcta también en worktrees.
- Que los proyectos ya existentes no se rompan mientras no se regeneren.

**Non-Goals:**
- Consumir las etiquetas: eso es `hm list` y el TUI.
- Estampar información que caduca (ver decisión 2).

## Decisions

### 1. Interpolación en lugar de sustitución con `sed`

Las etiquetas se escriben en la plantilla como `${HM_PROJECT:-}` y las resuelve Compose en
cada `up`. Alternativa descartada: sustituir con `sed` al generar, como se hace hoy con las
versiones de imagen. Se descarta porque el `docker-compose.yml` está versionado: hornear el
valor obligaría a regenerar (y a ensuciar el diff) cada vez que cambia algo, y haría que
todos los worktrees compartieran el valor del checkout donde se generó.

Las versiones de imagen siguen con `sed` porque sí son estables por proyecto.

### 2. Sólo se estampa lo que no caduca

Un contenedor vive días o semanas; la rama de git cambia cada pocas horas. Una etiqueta
`hm.branch` estaría mintiendo la mayor parte del tiempo.

Se estampa **identidad estable**: `hm.project` (nombre del proyecto Compose), `hm.root`
(ruta absoluta en el host del checkout desde el que se levantó), `hm.worktree` (slug del
worktree, vacío si es el checkout principal), `hm.profile`, `hm.magento`, `hm.version`
(versión de `hm` que lo creó) y `hm.agent` (opcional, lo estampa el flujo de agentes).

Lo **volátil** —rama actual, estado de git, última actividad— se deriva en tiempo de
lectura a partir de `hm.root`, que es una ruta de host: quien lee puede ejecutar
`git -C "$hm_root" rev-parse --abbrev-ref HEAD`. Esto además permite detectar
**huérfanos**: si `hm.root` ya no existe en disco, el entorno sobra.

### 3. Etiquetas en todos los servicios, no sólo en uno

Cuesta lo mismo y evita casos raros: si sólo `phpfpm` llevara las etiquetas, un entorno con
`phpfpm` parado desaparecería del inventario.

### 4. Retroceso para entornos antiguos

La función de descubrimiento busca primero `label=hm.project`; si no encuentra nada, cae a
`label=com.docker.compose.project` combinado con la comprobación que ya existe hoy
(`x-toolname: hiberus-magento` en el fichero compose). Así un entorno creado con una
versión anterior sigue apareciendo, marcado como "sin metadatos".

### 5. Nombres de etiqueta

Prefijo `hm.` en lugar de un dominio invertido tipo `com.hiberus.dockergento.`. Es la
convención que ya usan Compose (`com.docker.compose.*`) y Traefik (`traefik.*`) en el
sentido de ser cortas y legibles en la línea de comandos, y estas etiquetas se van a
escribir a mano en muchos `docker ps --filter`.

## Risks / Trade-offs

- **Un proyecto no regenerado no tiene etiquetas** → retroceso de la decisión 4; se
  documenta que `hm setup -f` las incorpora.
- **`hm.root` queda obsoleta si se mueve el directorio del proyecto** → es precisamente la
  señal que se quiere: un entorno cuyo `hm.root` no existe es candidato a limpieza, y quien
  lo consuma debe presentarlo como aviso, no como error.
- **Fugas de información en la etiqueta de ruta** → son rutas locales de desarrollo en la
  máquina del propio desarrollador; no se publican en ningún sitio.
- **Interpolación con variables no definidas** → todas llevan valor por defecto vacío
  (`${VAR:-}`) para que `docker compose config` no falle ni emita avisos.
- **mac y linux**: sin diferencias. Los overlays específicos de plataforma no necesitan
  etiquetas propias porque se fusionan sobre los mismos servicios.

## Migration Plan

1. Se publica con una versión normal de `hm`.
2. Los proyectos nuevos las llevan desde el `setup`.
3. Los existentes las incorporan cuando ejecuten `hm setup -f` y recreen contenedores; se
   documenta en el changelog y no se fuerza.
4. Mientras tanto siguen siendo descubribles por el camino de retroceso.

Reversión: quitar el bloque de la plantilla. Las etiquetas de los contenedores ya creados
se van con ellos en la siguiente recreación.

## Open Questions

- ¿`hm.profile` debe existir ya, si los perfiles (`lite`/`agent`/`full`) no llegan hasta
  WT-02? Propuesta: sí, con valor `full` por defecto, para no tener que volver a tocar la
  plantilla ni recrear contenedores más adelante.
