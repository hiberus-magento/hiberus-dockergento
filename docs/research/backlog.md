# Backlog de evolución de Hiberus Dockergento

> Consolida todo lo propuesto en
> [ddev-warden-feature-mining.md](ddev-warden-feature-mining.md) ("cantera"),
> [control-plane-ui.md](control-plane-ui.md) ("torre de control") y las dependencias
> relevantes de [git-worktrees.md](git-worktrees.md) y [ai-features.md](ai-features.md).
>
> **Nada de esto está implementado.** Este documento es la lista de la que se va eligiendo.

---

## Cómo usar este backlog

- Cada ítem tiene **ID estable** (no se reutiliza), esfuerzo, dependencias y criterios de
  aceptación. Los IDs se citan en ramas, commits y PRs.
- **Esfuerzo**: `S` ≈ 1-2 días · `M` ≈ 3-5 días · `L` ≈ más de una semana.
- **Estado**: `backlog` → `spec` → `wip` → `hecho`. Al pasar a `spec` se crea la carpeta
  en `specs/` con speckit y se enlaza desde aquí.
- Un ítem no entra en `wip` sin criterios de aceptación cerrados.

## Decisiones ya tomadas

| # | Decisión | Motivo |
|---|---|---|
| D1 | **Seguimos con Dockergento**, no migramos a Warden ni DDEV | Es la herramienta de todo el departamento y está 100 % enfocada a Magento 2 / Adobe Commerce |
| D2 | **TUI antes que web** | La GUI oficial de DDEV está abandonada; su TUI sigue vivo. Primero validamos cuánto valor está en *ver* la flota |
| D3 | **`share` con Cloudflared, no ngrok** | ngrok ya ha estado bloqueado en la empresa |
| D4 | **`hm` no llama a modelos de IA** | Coste, dependencia de proveedor y superficie de seguridad; ya lo hacen Claude Code, Cursor o Codex |
| D5 | **Sin registro de add-ons ni env-types multi-framework** | Coste de ecosistema que no nos corresponde; nuestro valor es el foco en Magento |
| D6 | **Sin Portainer como servicio global** | Aporta poco frente a Docker Desktop y es superficie que mantener |
| D7 | **La web no implementará lógica de negocio** | Sólo consume el contrato JSON y llama a los mismos verbos que la CLI |

---

## Estado actual

Cinco cambios de OpenSpec creados con proposal, design, specs y tasks
(`openspec list` para verlos). Ninguno implementado todavía.

| Change | Cubre | Tareas |
|---|---|---|
| [polish-terminal-ux](../../openspec/changes/polish-terminal-ux/) | UX-01, UX-02, UX-03 | 17 |
| [terminal-components](../../openspec/changes/terminal-components/) | UX-07 | 25 |
| [add-cli-output-contract](../../openspec/changes/add-cli-output-contract/) | CLI-01 | 32 |
| [add-compose-project-labels](../../openspec/changes/add-compose-project-labels/) | ENV-02 | 21 |
| [add-describe-and-list-commands](../../openspec/changes/add-describe-and-list-commands/) | CLI-02, CLI-03 | 26 |
| [add-doctor-command](../../openspec/changes/add-doctor-command/) | CLI-04 | 29 |
| [add-worktree-guardrails](../../openspec/changes/add-worktree-guardrails/) | WT-01 | 25 |

## Vista de triaje

| ID | Ítem | Área | Esfuerzo | Depende de | Estado |
|---|---|---|---|---|---|
| **CLI-01** | Modo agente: `--json`, `--yes`, códigos de salida | CLI | M | — | **hecho** |
| **CLI-02** | `hm describe [--json]` | CLI | S | CLI-01 | **hecho** |
| **CLI-03** | `hm list [--json]` | CLI | S | CLI-01, ENV-02 | **hecho** |
| **CLI-04** | `hm doctor` | CLI | M | — | **hecho** |
| **CLI-05** | `hm logs [servicio] [-f]` | CLI | S | — | backlog |
| **CLI-06** | `hm launch` | CLI | S | CLI-02 | backlog |
| **CLI-07** | `hm version` | CLI | S | — | backlog |
| **CLI-08** | `hm clean [--dry-run]` | CLI | S | ENV-02 | backlog |
| **CLI-09** | Lanzadores de clientes de BD | CLI | S | CLI-02 | backlog |
| **PERF-01** | `hm --help` en una sola pasada de `jq` | Rendimiento | S | — | **hecho** |
| **PERF-02** | Coste fijo de arranque perezoso | Rendimiento | M | — | **hecho** |
| **PERF-03** | `hm doctor` en paralelo | Rendimiento | S | — | **hecho** |
| **PERF-04** | Presupuesto de rendimiento vigilado por test | Rendimiento | S | PERF-01 | **hecho** |
| **UX-01** | Contraseñas sin eco en los prompts | UX | S | — | **hecho** |
| **UX-02** | Honrar `NO_COLOR`, `TERM=dumb` y `--no-color` | UX | S | — | **hecho** |
| **UX-03** | Dejar de borrar la pantalla al preguntar | UX | S | — | **hecho** |
| **UX-04** | Ayuda agrupada, con uso y ejemplos | UX | M | — | backlog |
| **UX-05** | Señal en operaciones largas (<100 ms) | UX | M | UX-02 | backlog |
| **UX-06** | Selector navegable con flechas | UX | M | UX-07 | backlog |
| **UX-07** | Biblioteca de componentes de terminal | UX | M | — | [spec](../../openspec/changes/terminal-components/) |
| **TUI-01** | Dashboard de terminal (`hm` sin argumentos) | TUI | M | CLI-02, CLI-03, UX-07 | backlog |
| **NET-01** | Proxy global (`hm proxy`) con Traefik | Red | L | ENV-02 | backlog |
| **NET-02** | Certificado wildcard | Red | S | NET-01 | backlog |
| **NET-03** | dnsmasq: fin de `/etc/hosts` | Red | M | NET-01 | backlog |
| **NET-04** | `hm share` con Cloudflared | Red | S | NET-01 | backlog |
| **ENV-01** | Nombre de proyecto derivado del directorio | Entorno | S | — | backlog |
| **ENV-02** | Etiquetas `hm.*` en la plantilla compose | Entorno | S | — | **hecho** |
| **ENV-03** | Mailpit en lugar de Mailhog | Entorno | S | — | backlog |
| **ENV-04** | Servicios opcionales (`--with=...`) | Entorno | M | — | backlog |
| **ENV-05** | Servicio de cron | Entorno | S | ENV-04 | backlog |
| **ENV-06** | Adminer / cliente web de BD | Entorno | S | ENV-04 | backlog |
| **ENV-07** | Selenium y Playwright (MFTF, e2e, agentes) | Entorno | M | ENV-04 | backlog |
| **ENV-08** | Mutagen en macOS | Entorno | L | — | backlog |
| **ENV-09** | Perfilado: XHProf / XHGui | Entorno | M | ENV-04 | backlog |
| **ENV-10** | Perfilado: Blackfire | Entorno | M | ENV-04 | backlog |
| **DB-01** | `hm db snapshot` / `restore` / `list` | Datos | M | — | backlog |
| **DB-02** | Volumen "golden" y clonado | Datos | M | DB-01 | backlog |
| **DB-03** | Ciclo de vida seguro (`stop --snapshot`, protección de `down -v`) | Datos | S | DB-01 | backlog |
| **DB-04** | Anonimización por defecto en entornos de agente | Datos | S | — | backlog |
| **INST-01** | Bootstrap: admin aleatorio y 2FA con QR | Instalación | S | — | backlog |
| **INST-02** | Flags `--clean-install` / `--db-dump` | Instalación | S | INST-01 | backlog |
| **INST-03** | Proveedores `hm pull` / `hm push` | Instalación | M | CLI-01 | backlog |
| **AI-01** | `hm verify [--changed] [--json]` | IA | M | CLI-01 | backlog |
| **AI-02** | Permisos y guardarraíles generados | IA | S | — | backlog |
| **AI-03** | `hm mcp` (sólo lectura) | IA | M | CLI-01, CLI-02 | backlog |
| **AI-04** | `hm mcp` (escritura acotada) | IA | M | AI-03, AI-02 | backlog |
| **AI-05** | Generación de `CLAUDE.md` / `AGENTS.md` y `.mcp.json` | IA | S | CLI-02 | backlog |
| **AI-06** | Fichero de exclusión de contexto | IA | S | — | backlog |
| **AI-07** | `hm ai-doctor` y versionado de skills | IA | S | — | backlog |
| **AI-08** | Índice del código como MCP | IA | L | AI-03 | backlog |
| **WT-01** | Guardarraíles de worktree | Worktrees | S | — | **hecho** |
| **WT-02** | `hm worktree` con perfiles de entorno | Worktrees | L | NET-01, DB-02 | backlog |
| **WT-03** | Recolección de worktrees huérfanos | Worktrees | S | WT-02, ENV-02 | backlog |
| **UI-01** | Dashboard web sólo lectura | Web | L | NET-01, CLI-02, CLI-03 | backlog |
| **UI-02** | Acciones seguras desde la web | Web | M | UI-01 | backlog |
| **UI-03** | Vista de worktrees y agentes | Web | M | UI-01, WT-02 | backlog |
| **UI-04** | Snapshots y acciones destructivas desde la web | Web | M | UI-02, DB-01 | backlog |

### Orden sugerido

1. **Ola 1 — cimientos (sin decisiones pendientes):** CLI-01, CLI-02, CLI-03, ENV-02, CLI-04.
2. **Ola 2 — valor inmediato barato:** CLI-05, CLI-06, CLI-07, ENV-03, ENV-01, INST-01, WT-01.
3. **Ola 3 — rendimiento y visibilidad:** PERF-01, PERF-03, PERF-02, PERF-04 *(hechos)*,
   UX-01, UX-02, UX-03 *(las tres pequeñas)*, CLI-08, DB-01, DB-03.
4. **Ola 3b — cimientos de interfaz:** UX-07, UX-04, UX-05, UX-06 y sólo entonces TUI-01.
5. **Ola 4 — el salto:** NET-01, NET-02, NET-03, NET-04.
6. **Ola 5 — agentes:** AI-01, AI-02, DB-02, WT-02, AI-03.
7. **Ola 6 — lo demás**, según lo que pida el equipo.

---

## Detalle

### Área CLI

#### CLI-01 · Modo agente: `--json`, `--yes`, códigos de salida
**Origen**: cantera §IA 3.1. **Esfuerzo**: M. **Depende de**: —.
**Problema**: la CLI asume terminal interactiva. Ya nos ha mordido: hasta la 1.4.5,
`hm mysql` trataba cualquier stdin sin tty como un volcado a importar, así que
`hm mysql -q "..."` desde CI o desde un agente nunca llegaba al parser de opciones.
**Propuesta**: flag `--json` en comandos de lectura, con JSON por defecto cuando stdout no
es un TTY; `--yes` y `HM_NON_INTERACTIVE=1`; auditoría de todos los `[ -t 0 ]` y `read -rp`;
errores estructurados por stderr.
**Aceptación**:
- Ningún comando se bloquea esperando entrada con `HM_NON_INTERACTIVE=1`.
- Códigos de salida distintos y documentados para: Docker parado, proyecto sin configurar,
  servicio caído, argumentos inválidos.
- Existe un test que ejecuta cada comando con stdin cerrado y sin tty.

#### CLI-02 · `hm describe [--json]`
**Origen**: cantera §3.2 + IA 3.2 (es el "contrato"). **Esfuerzo**: S. **Depende de**: CLI-01.
**Propuesta**: URL y dominio, nombre de proyecto compose, versión de Magento y de cada
servicio, nombres de contenedor, credenciales de BD, estado de Xdebug, rutas y modo de
despliegue.
**Aceptación**:
- Salida JSON con esquema versionado (`schema_version`).
- Las credenciales **no** aparecen salvo `--with-secrets`.
- Funciona con el entorno parado (indicando el estado), no sólo levantado.

#### CLI-03 · `hm list [--json]`
**Origen**: cantera §3.2. **Esfuerzo**: S. **Depende de**: CLI-01, ENV-02.
**Propuesta**: todos los proyectos Dockergento de la máquina, su estado, URL, perfil y
worktrees asociados, descubiertos por etiquetas.
**Aceptación**: lista proyectos levantados y parados; distingue checkout principal de
worktrees; no depende del directorio desde el que se ejecute.

#### CLI-04 · `hm doctor`
**Origen**: cantera §3.4. **Esfuerzo**: M. **Depende de**: —.
**Problema**: hoy hay un único mensaje genérico ("Docker is not properly configured…
execute hm setup") que sale igual si Docker está parado, si falta el fichero compose o si
el YAML es inválido.
**Propuesta**: comprobaciones de demonio, versión de compose, puertos ocupados **y qué
proceso los ocupa**, validez del compose, certificados y confianza del sistema, entradas de
`/etc/hosts`, espacio de volúmenes y estado por servicio.
**Aceptación**: cada comprobación devuelve OK / aviso / error con **una acción concreta**;
`--json` para el dashboard; termina en menos de 5 s.

#### CLI-05 · `hm logs [servicio] [-f]`
**Esfuerzo**: S. Envoltorio de `docker compose logs` acotado al proyecto, con `--since` y
`--tail`. Útil para personas y para agentes.

#### CLI-06 · `hm launch`
**Esfuerzo**: S. Abre la URL del proyecto (y `--admin`, `--mailpit`, `--rabbitmq`).

#### CLI-07 · `hm version`
**Esfuerzo**: S. Hoy **no hay forma de saber qué versión de Dockergento tienes**. Debe
mostrar versión de la CLI, commit, versión de Docker y de Compose. Requiere fijar de dónde
sale la versión (tag de git frente a fichero `VERSION`).

#### CLI-08 · `hm clean [--dry-run]`
**Origen**: cantera §3.11. **Esfuerzo**: S. **Depende de**: ENV-02.
**Dato**: en una máquina del equipo hay **152 volúmenes y 69 GB**, 45 imágenes y 81
contenedores. **Aceptación**: sólo toca recursos con etiqueta `hm.*`; `--dry-run` por
defecto; nunca borra volúmenes de proyectos existentes sin `--force`.

#### CLI-09 · Lanzadores de clientes de BD
**Esfuerzo**: S. `hm tableplus` / `hm sequelace` / `hm dbeaver` abriendo el cliente ya
conectado, al estilo de DDEV.

### Área Rendimiento

#### PERF-01 · `hm --help` en una sola pasada de `jq`
**Esfuerzo**: S. **Depende de**: —.
**Medido**: `hm --help` tarda **5,7–6,2 s** y lanza **143 procesos `jq`** —tres por cada uno
de los 45 comandos— sobre el mismo fichero, que cabe entero en memoria. Es lo primero que
ejecuta quien se acerca a la herramienta.
**Objetivo**: por debajo de 500 ms con una sola invocación de `jq`.

#### PERF-02 · Coste fijo de arranque perezoso
**Esfuerzo**: M. Cada invocación paga hoy `docker compose version` (188 ms), la versión de
Magento leyendo un `composer.lock` de 1,6 MB (77 ms) y `git describe` (56 ms), aunque el
comando no los use. Se calculan la primera vez que se piden y se memorizan.

#### PERF-03 · `hm doctor` en paralelo
**Esfuerzo**: S. Doce comprobaciones independientes ejecutadas en serie: **3,9–4,6 s**. En
paralelo el peor caso es la más lenta, no la suma. Objetivo: por debajo de 2 s.

#### PERF-04 · Presupuesto de rendimiento vigilado por test
**Esfuerzo**: S. **Depende de**: PERF-01. Una prueba que falla si `hm --help`, el arranque
mínimo o `hm doctor` se pasan de presupuesto, para que la mejora no se degrade en silencio.

### Área UX

Todo el detalle en [terminal-ux.md](terminal-ux.md).

#### UX-01 · Contraseñas sin eco
**Esfuerzo**: S. `hm transfer-db` pide la contraseña de la base de datos con `read -p`, sin
`-s`: se ve en pantalla y queda en el scrollback. clig.dev lo prohíbe explícitamente y es lo
único de esta área que además es un problema de seguridad.

#### UX-02 · Honrar los estándares de color
**Esfuerzo**: S. No se respeta `NO_COLOR` (que sí respetan Docker, git, ripgrep y compañía),
ni `TERM=dumb`, ni existe `--no-color`. Como los colores ya salen de variables, basta un
único punto de decisión en `load_colors`.

#### UX-03 · Dejar de borrar la pantalla al preguntar
**Esfuerzo**: S. `custom_question` y `custom_select` llaman a `clear`, así que en `hm setup`
cada respuesta borra lo que el usuario acaba de leer. El borrado tiene sentido en un TUI, y
allí se hace entrando al buffer alternativo, que se puede deshacer al salir.

#### UX-04 · Ayuda agrupada, con uso y ejemplos
**Esfuerzo**: M. Hoy son 45 comandos en una lista alfabética plana, sin línea de uso y sin
ejemplos. clig.dev pide empezar por ejemplos y poner lo más común primero; `docker`, `gh` y
`ddev` agrupan por propósito. La clasificación ya existe del contrato de salida: falta usarla
para presentar, con los grupos declarados en `command_descriptions.json`.

#### UX-05 · Señal en operaciones largas
**Esfuerzo**: M. `composer install` y `setup:upgrade` pueden estar minutos sin decir nada.
Lo relevante no es el spinner: es la regla de imprimir algo antes de 100 ms. Sin animaciones
cuando no hay TTY.

#### UX-06 · Selector navegable con flechas
**Esfuerzo**: M. Hoy es el `select` de Bash: lista numerada, escribir un número, sin valor
por defecto. Con retroceso a la lista actual y a `fzf` si está instalado.

#### UX-07 · Biblioteca de componentes de terminal
**Esfuerzo**: M. Tamaño con `stty size` (POSIX, funciona en el Bash 3.2 de macOS), cursor,
buffer de pantalla alternativo, `SIGWINCH` y lectura de teclas, todo con secuencias VT100 en
crudo y sin `tput`. **Es el prerrequisito del TUI**: sin esto, el TUI no sabe ni cuánto mide
la ventana ni cómo salir sin destruir el scrollback.

### Área TUI

#### TUI-01 · Dashboard de terminal (`hm` sin argumentos)
**Origen**: torre §9 y decisión D2. **Esfuerzo**: M. **Depende de**: CLI-02, CLI-03.
**Propuesta**: ejecutar `hm` sin argumentos abre una vista de la flota: proyectos, estado,
URL, worktrees y avisos del doctor, con atajos para arrancar, parar, ver logs y abrir.
**Notas de diseño**:
- **No necesita backend, ni servidor HTTP, ni socket expuesto, ni imagen nueva**: corre en
  el host dentro de la propia CLI.
- Sin dependencias obligatorias: bash + ANSI. Si `fzf` está instalado, usarlo para la
  selección; si no, navegación con teclas.
- Consume el mismo JSON que consumirá la web: el trabajo no se tira.
**Aceptación**: refresca sin parpadeo; funciona en terminal de 80 columnas; `Ctrl-C` sale
limpio; si no hay proyectos, explica cómo crear uno.

### Área Red

#### NET-01 · Proxy global (`hm proxy up|down|status`)
**Origen**: cantera §3.1. **Esfuerzo**: L. **Depende de**: ENV-02.
**Problema**: cada proyecto publica 80, 443, 3306, 9200, 8025, 5672 y 15672, así que **dos
proyectos Dockergento no pueden estar levantados a la vez**.
**Propuesta**: stack global con Traefik enrutando por `Host`; los proyectos dejan de
publicar puertos.
**Aceptación**: dos proyectos simultáneos accesibles por sus dominios; `hm proxy status`
muestra qué rutas hay; un proyecto arrancado sin proxy sigue funcionando (compatibilidad).
**Riesgo**: es el cambio más invasivo del backlog; necesita plan de migración para los
proyectos existentes.

#### NET-02 · Certificado wildcard
**Esfuerzo**: S. **Depende de**: NET-01. Un `mkcert` para `*.<dominio>` en lugar de uno por
proyecto, como hace `warden sign-certificate`.

#### NET-03 · dnsmasq
**Esfuerzo**: M. **Depende de**: NET-01. Resolver `*.local` (o `*.test`) sin escribir en
`/etc/hosts` con `sudo`, que es lo que hace hoy `set-host.sh`. **Ojo**: en macOS requiere
un resolvedor en `/etc/resolver/`, y `.local` colisiona con mDNS/Bonjour — **valorar
cambiar a `.test`**, que es el TLD reservado que usan Warden y DDEV.

#### NET-04 · `hm share` con Cloudflared
**Origen**: cantera §3.5. **Decisión D3**. **Esfuerzo**: S.
**Casos de uso**: enseñar avances a cliente o QA sin desplegar, y **recibir webhooks
reales** de pasarelas de pago, ERPs y marketplaces contra el entorno local.
**Aceptación**: `hm share` devuelve una URL pública; avisa de que el entorno queda expuesto
y exige confirmación; `hm share --stop`; documenta que la URL cambia en cada arranque salvo
que se configure un túnel con nombre.

### Área Entorno

#### ENV-01 · Nombre de proyecto derivado del directorio
**Origen**: cantera §3.12. **Esfuerzo**: S. DDEV lo ofrece como opción global precisamente
para que **cada worktree sea un proyecto distinto sin configurar nada**. Requiere decidir
qué pasa con el `COMPOSE_PROJECT_NAME` que hoy está versionado en `properties.json`.

#### ENV-02 · Etiquetas `hm.*` en la plantilla compose
**Origen**: torre §5.2. **Esfuerzo**: S.
`hm.project`, `hm.worktree`, `hm.branch`, `hm.agent`, `hm.profile`, `hm.magento`.
Es la base de `list`, `clean`, el TUI, el dashboard y la detección de huérfanos. **Sin
estado que se desincronice**: nada de fichero de registro.

#### ENV-03 · Mailpit en lugar de Mailhog
**Esfuerzo**: S. **Mailhog está sin mantenimiento**; Warden migró a Mailpit en v0.15 y
nuestra plantilla sigue con Mailhog. Mantener compatibilidad de puerto/UI y actualizar
`requirements.json` y la documentación.

#### ENV-04 · Servicios opcionales (`--with=...`)
**Esfuerzo**: M. Perfiles opcionales en la plantilla en lugar de un registro de add-ons
(D5). Habilita ENV-05 a ENV-10.

#### ENV-05 · Servicio de cron
**Esfuerzo**: S. **Hoy no hay servicio de cron**, que es una diferencia real con producción
en cualquier proyecto Magento.

#### ENV-06 · Adminer
**Esfuerzo**: S. Acceso web a la BD sin cliente instalado; útil para dar acceso puntual.

#### ENV-07 · Selenium y Playwright
**Esfuerzo**: M. Cubre dos necesidades a la vez: **MFTF/e2e** para el equipo y
**verificación visual para agentes** (cantera §IA 3.5). Debe resolver por DNS interno
(`nginx`), no por `localhost`.

#### ENV-08 · Mutagen en macOS
**Esfuerzo**: L. Sustituye el ciclo actual de `copy-to-container` → parar → `docker cp` de
vuelta en cada `composer install`, origen de la mayoría de rarezas en Mac. Es el mayor
salto de DX disponible y también el más invasivo. **Prototipo antes de comprometerse.**

#### ENV-09 · XHProf / XHGui  ·  #### ENV-10 · Blackfire
**Esfuerzo**: M cada uno. El rendimiento es *el* problema recurrente de Adobe Commerce y
hoy sólo tenemos Xdebug. Empezar por XHProf (libre) y valorar Blackfire (licencia).

### Área Datos

#### DB-01 · `hm db snapshot` / `restore` / `list`
**Origen**: cantera §3.3. **Esfuerzo**: M.
**Propuesta**: snapshots con **backup en caliente** (`mariadb-backup`) dentro del
contenedor, con nombre y fecha, al estilo de `ddev snapshot`.
**Aceptación**: crear y restaurar sin parar el proyecto; `--name`; lista con tamaño y
origen; funciona con las versiones de MariaDB de `requirements.json`.

#### DB-02 · Volumen "golden" y clonado
**Origen**: worktrees §4.2. **Esfuerzo**: M. **Depende de**: DB-01.
Clonar el volumen de datos desde una plantilla congelada para levantar entornos aislados en
segundos sin tocar el principal. Medido: `dbdata` 245 MB y `workspace` 695 MB en un
proyecto real, o sea clones de segundos.

#### DB-03 · Ciclo de vida seguro
**Esfuerzo**: S. `hm stop --snapshot`, confirmación en `down -v`, y acotar
`docker-stop-all` al proyecto o exigir confirmación explícita.

#### DB-04 · Anonimización por defecto en entornos de agente
**Esfuerzo**: S. Apoyado en `hm masquerade`, que **ya existe**. Con agentes, la
anonimización deja de ser buena práctica y pasa a ser cumplimiento.

### Área Instalación

#### INST-01 · Admin aleatorio y 2FA con QR
**Origen**: cantera §3.9. **Esfuerzo**: S.
Hoy el admin sale de `data/config.json` con contraseña fija `Hiberus123` y sin resolver el
2FA, que es el primer tropiezo de cualquier instalación moderna. Warden genera contraseña
aleatoria y pinta el QR de Google Authenticator en el terminal.

#### INST-02 · `--clean-install` / `--db-dump`
**Esfuerzo**: S. Alinear `hm setup`/`hm install` con las opciones del bootstrap de Warden.

#### INST-03 · Proveedores `hm pull` / `hm push`
**Esfuerzo**: M. Formalizar `transfer-db`, `transfer-media` y `cloud` como proveedores
declarativos (Adobe Commerce Cloud, SSH/rsync, S3), ejecutables sin interacción.

### Área IA

#### AI-01 · `hm verify [--changed] [--json]`
**Esfuerzo**: M. **Depende de**: CLI-01.
PHPCS con estándar Magento2, PHPStan/Rector, `test-unit`, compilación de DI y validación de
XML/XSD, con salida estructurada. Sirve como comando manual, como hook de cierre de tarea
de los agentes y en CI. **El cuello de botella con agentes no es generar, es verificar.**

#### AI-02 · Permisos y guardarraíles generados
**Esfuerzo**: S. Clasificar cada comando como *seguro sin supervisión* o *requiere
confirmación* y generar desde ahí la configuración de permisos de cada plataforma. Hoy cada
persona del equipo mantiene esa lista a mano y todas distintas.

#### AI-03 · `hm mcp` (sólo lectura)  ·  #### AI-04 · `hm mcp` (escritura acotada)
**Esfuerzo**: M cada uno. Herramientas tipadas: lectura (describe, list, logs, `db.query`
sólo SELECT, estado de índices) y escritura acotada (cache clean/flush, reindex,
config:set). Las peligrosas (setup:upgrade, composer update, import de BD, `down -v`)
quedan fuera o exigen confirmación humana.

#### AI-05 · Generación de `CLAUDE.md` / `AGENTS.md` y `.mcp.json`
**Esfuerzo**: S. **Depende de**: CLI-02. Que el agente no tenga que adivinar —ni inventarse—
URLs, nombres de contenedor o versiones.

#### AI-06 · Fichero de exclusión de contexto
**Esfuerzo**: S. `app/etc/env.php`, `var/log/*`, `pub/media/customer/*`, `vendor/`,
`generated/`, `var/cache/`.

#### AI-07 · `hm ai-doctor` y versionado de skills
**Esfuerzo**: S. Qué skills hay, de qué repositorio, qué versión y si están al día. Hoy
`ai-pull --force` va a ciegas y se sigue una rama, no una versión.

#### AI-08 · Índice del código como MCP
**Esfuerzo**: L. Módulos, plugins, preferencias, observers y layouts indexados para que los
agentes no quemen contexto. **Prototipo antes de decidir.**

### Área Worktrees

#### WT-01 · Guardarraíles de worktree
**Origen**: worktrees §6 fase 1. **Esfuerzo**: S.
**Problema demostrado**: ejecutar `hm start` o `hm rebuild` desde un worktree **re-apunta
los bind mounts del entorno principal al worktree**, y `hm down -v` lo destruye con su BD.
**Propuesta**: detectar worktree (`git rev-parse --git-common-dir`), resolver rutas contra
el checkout principal y bloquear los comandos que alteran la topología salvo `--force`.
**Es urgente e independiente de todo lo demás.**

#### WT-02 · `hm worktree` con perfiles de entorno
**Esfuerzo**: L. **Depende de**: NET-01, DB-02. Perfiles `lite` (sólo PHP), `agent`
(php+nginx+db+search+redis) y `full`.

#### WT-03 · Recolección de huérfanos
**Esfuerzo**: S. Contenedores y volúmenes cuyo worktree ya no existe en git.

### Área Web

> Toda esta área queda **después del TUI** (D2) y de NET-01.

#### UI-01 · Dashboard web sólo lectura
**Esfuerzo**: L. Servicio del stack global en `https://hm.test/`, distribuido como imagen
`hiberusmagento/dashboard` y versionado junto a la CLI.
**Requisitos de seguridad no negociables**: validación de la cabecera `Host` (DNS
rebinding), tokens CSRF, el socket de Docker nunca expuesto al navegador, escucha en
loopback, y **arranque en sólo lectura**.
**Decisión pendiente**: PHP + htmx/Alpine (mantenible por el departamento) frente a binario
Go (lo que hacen DDEV y Warden).

#### UI-02 · Acciones seguras
**Esfuerzo**: M. start/stop/restart, logs, abrir, cache y reindex. Se habilitan con
`--with-actions`. **Frontera**: lo que se puede hacer por el socket de Docker lo hace la
web; lo que toca el sistema de ficheros del host se queda en la CLI, y la web muestra el
comando listo para copiar.

#### UI-03 · Vista de worktrees y agentes
**Esfuerzo**: M. Rama, tarea, agente, perfil, estado, última actividad y huérfanos. Es
**el** plano de control multi-agente.

#### UI-04 · Snapshots y acciones destructivas
**Esfuerzo**: M. Con confirmación escrita y registro en la vista de actividad.
