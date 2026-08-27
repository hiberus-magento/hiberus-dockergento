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
| D8 | **El TLD lo elige cada proyecto**, `.local` por defecto | Cambiarlo por decreto obligaría a regenerar dominios y certificados en todos los proyectos existentes |

---

## Estado actual

Los cambios de OpenSpec creados a partir de este backlog, todos ya implementados y
archivados en `openspec/changes/archive/`. Las áreas de Red, Base de datos,
Instalación e IA siguen enteras en `backlog`.

| Change | Cubre | Tareas | Estado |
|---|---|---|---|
| [polish-terminal-ux](../../openspec/changes/archive/) | UX-01, UX-02, UX-03 | 17 | hecho |
| [terminal-components](../../openspec/changes/archive/) | UX-07 | 25 | hecho |
| [terminal-dashboard](../../openspec/changes/archive/) | TUI-01 | 35 | hecho |
| [add-cli-output-contract](../../openspec/changes/archive/) | CLI-01 | 32 | hecho |
| [add-compose-project-labels](../../openspec/changes/archive/) | ENV-02 | 21 | hecho |
| [add-describe-and-list-commands](../../openspec/changes/archive/) | CLI-02, CLI-03 | 26 | hecho |
| [add-doctor-command](../../openspec/changes/archive/) | CLI-04 | 29 | hecho |
| [add-worktree-guardrails](../../openspec/changes/archive/) | WT-01 | 25 | hecho |
| [smooth-dashboard-rendering](../../openspec/changes/archive/) | TUI-01 (render) | 31 | hecho |
| [add-logs-launch-version](../../openspec/changes/archive/) | CLI-05, CLI-06, CLI-07 | 35 | hecho |
| [derive-project-name](../../openspec/changes/archive/) | ENV-01 | 26 | hecho |
| [choose-mail-catcher](../../openspec/changes/archive/) | ENV-03 | 28 | hecho |
| [bootstrap-admin-credentials](../../openspec/changes/archive/) | INST-01 | 25 | hecho |
| [add-database-snapshots](../../openspec/changes/archive/) | DB-01 | 33 | hecho |
| [protect-environment-lifecycle](../../openspec/changes/archive/) | DB-03 | 28 | hecho |
| [add-clean-command](../../openspec/changes/archive/) | CLI-08 | 28 | hecho |
| [speed-up-cli](../../openspec/changes/archive/) | PERF-01..04 | 34 | hecho |
| [grouped-help](../../openspec/changes/archive/) | UX-04 | 27 | hecho |
| [version-switching](../../openspec/changes/archive/) | REL-01, REL-02 | 29 | hecho |

## Vista de triaje

| ID | Ítem | Área | Esfuerzo | Depende de | Estado |
|---|---|---|---|---|---|
| **CLI-01** | Modo agente: `--json`, `--yes`, códigos de salida | CLI | M | — | **hecho** |
| **CLI-02** | `hm describe [--json]` | CLI | S | CLI-01 | **hecho** |
| **CLI-03** | `hm list [--json]` | CLI | S | CLI-01, ENV-02 | **hecho** |
| **CLI-04** | `hm doctor` | CLI | M | — | **hecho** |
| **CLI-05** | `hm logs [servicio] [-f]` | CLI | S | — | **hecho** |
| **CLI-06** | `hm launch` | CLI | S | CLI-02 | **hecho** |
| **CLI-07** | `hm version` | CLI | S | — | **hecho** |
| **CLI-08** | `hm clean [--dry-run]` | CLI | S | ENV-02 | **hecho** |
| **CLI-09** | Lanzadores de clientes de BD | CLI | S | CLI-02 | backlog |
| **PERF-01** | `hm --help` en una sola pasada de `jq` | Rendimiento | S | — | **hecho** |
| **PERF-02** | Coste fijo de arranque perezoso | Rendimiento | M | — | **hecho** |
| **PERF-03** | `hm doctor` en paralelo | Rendimiento | S | — | **hecho** |
| **PERF-04** | Presupuesto de rendimiento vigilado por test | Rendimiento | S | PERF-01 | **hecho** |
| **REL-01** | `hm --version` con la referencia exacta | Release | S | — | **hecho** |
| **REL-02** | `hm switch` para cambiar de versión y volver | Release | M | REL-01 | **hecho** |
| **REL-03** | `hm update` no debe sacar de un tag en silencio | Release | S | — | **hecho** |
| **UX-01** | Contraseñas sin eco en los prompts | UX | S | — | **hecho** |
| **UX-02** | Honrar `NO_COLOR`, `TERM=dumb` y `--no-color` | UX | S | — | **hecho** |
| **UX-03** | Dejar de borrar la pantalla al preguntar | UX | S | — | **hecho** |
| **UX-04** | Ayuda agrupada, con uso y ejemplos | UX | M | — | **hecho** |
| **UX-05** | Señal en operaciones largas (<100 ms) | UX | M | UX-02 | backlog |
| **UX-06** | Selector navegable con flechas | UX | M | UX-07 | backlog |
| **UX-07** | Biblioteca de componentes de terminal | UX | M | — | **hecho** |
| **TUI-01** | Dashboard de terminal (`hm tui`) | TUI | M | CLI-02, CLI-03, UX-07 | **hecho** |
| **NET-01** | Proxy global (`hm proxy`) con Traefik | Red | L | ENV-02 | **hecho** |
| **NET-02** | Certificado wildcard | Red | S | NET-01 | **hecho** |
| **NET-03** | dnsmasq: fin de `/etc/hosts` | Red | M | NET-01 | **parcial** |
| **NET-04** | `hm share` con Cloudflared | Red | S | NET-01 | **hecho** |
| **ENV-01** | Nombre de proyecto derivado del directorio | Entorno | S | — | **hecho** |
| **ENV-02** | Etiquetas `hm.*` en la plantilla compose | Entorno | S | — | **hecho** |
| **ENV-03** | Mailpit en lugar de Mailhog | Entorno | S | — | **hecho** |
| **ENV-04** | Servicios opcionales (`--with=...`) | Entorno | M | — | backlog |
| **ENV-05** | Servicio de cron | Entorno | S | ENV-04 | backlog |
| **ENV-06** | Adminer / cliente web de BD | Entorno | S | ENV-04 | backlog |
| **ENV-07** | Selenium y Playwright (MFTF, e2e, agentes) | Entorno | M | ENV-04 | backlog |
| **ENV-08** | Mutagen en macOS | Entorno | L | — | backlog |
| **ENV-09** | Perfilado: XHProf / XHGui | Entorno | M | ENV-04 | backlog |
| **ENV-10** | Perfilado: Blackfire | Entorno | M | ENV-04 | backlog |
| **DB-01** | `hm db snapshot` / `restore` / `list` | Datos | M | — | **hecho** |
| **DB-02** | Volumen "golden" y clonado | Datos | M | DB-01 | backlog |
| **DB-03** | Ciclo de vida seguro (`stop --snapshot`, protección de `down -v`) | Datos | S | DB-01 | **hecho** |
| **DB-04** | Anonimización por defecto en entornos de agente | Datos | S | — | backlog |
| **INST-01** | Bootstrap: admin aleatorio y 2FA con QR | Instalación | S | — | **hecho** |
| **INST-02** | Flags `--clean-install` / `--db-dump` | Instalación | S | INST-01 | backlog |
| **INST-03** | Proveedores `hm pull` / `hm push` | Instalación | M | CLI-01 | backlog |
| **AI-01** | `hm verify [--changed] [--json]` | IA | M | CLI-01 | **hecho** |
| **AI-02** | Permisos y guardarraíles generados | IA | S | — | **hecho** |
| **AI-03** | `hm mcp` (sólo lectura) | IA | M | CLI-01, CLI-02 | **hecho** |
| **AI-04** | `hm mcp` (escritura acotada) | IA | M | AI-03, AI-02 | backlog |
| **AI-05** | Generación de `CLAUDE.md` / `AGENTS.md` y `.mcp.json` | IA | S | CLI-02 | backlog |
| **AI-06** | Fichero de exclusión de contexto | IA | S | — | backlog |
| **AI-07** | `hm ai-doctor` y versionado de skills | IA | S | — | backlog |
| **AI-08** | Índice del código como MCP | IA | L | AI-03 | backlog |
| **WT-01** | Guardarraíles de worktree | Worktrees | S | — | **hecho** |
| **WT-02** | `hm worktree` con perfiles de entorno | Worktrees | L | NET-01, DB-02 | **hecho** |
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
6. **Ola 5 — agentes:** AI-01, AI-02, DB-02, WT-02, AI-03 *(hechos)*.
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

#### CLI-05 · `hm logs [servicio] [-f]` — hecho
**Esfuerzo**: S. Envoltorio de `docker compose logs` acotado al proyecto, con `--since` y
`--tail`. Útil para personas y para agentes.
**Cómo quedó**: comando transparente, así que sus opciones llegan intactas a Compose y su salida
nunca se envuelve. Añade lo que Compose no da: un nombre de servicio inexistente se rechaza con
la lista de los que sí existen y código de salida 5. Documentado en [docs/logs.md](../logs.md).

#### CLI-06 · `hm launch` — hecho
**Esfuerzo**: S. Abre la URL del proyecto (y `--admin`, `--mailhog`, `--rabbitmq`, `--search`).
**Cómo quedó**: las direcciones salen de la misma fuente que `describe`. Donde no hay dónde
abrir —script, SSH, `--json`— escribe la dirección en lugar de fallar. Dos destinos a la vez son
error de uso, no dos pestañas. No arranca lo que está parado.
Documentado en [docs/launch.md](../launch.md).

#### CLI-07 · `hm version` — hecho
**Esfuerzo**: S. Hoy **no hay forma de saber qué versión de Dockergento tienes**. Debe
mostrar versión de la CLI, commit, versión de Docker y de Compose. Requiere fijar de dónde
sale la versión (tag de git frente a fichero `VERSION`).
**Cómo quedó**: la versión sale de `git describe`, no de un fichero `VERSION` — así no hay dos
fuentes que puedan discrepar (resuelto en REL-01). `hm --version` se deja intacto porque es el
camino más corto de la CLI y tiene presupuesto de rendimiento; el comando paga las dos llamadas
a Docker. No exige proyecto. Documentado en [docs/version.md](../version.md).

#### CLI-08 · `hm clean [--dry-run]` — hecho
**Origen**: cantera §3.11. **Esfuerzo**: S. **Depende de**: ENV-02.
**Motivo**: una máquina que ha levantado unos cuantos proyectos acumula con
facilidad más de cien volúmenes y decenas de GB que nadie va a reclamar. **Aceptación**: sólo toca recursos con etiqueta `hm.*`; `--dry-run` por
defecto; nunca borra volúmenes de proyectos existentes sin `--force`.
**Cómo quedó**: se invirtió el planteamiento — no hay `--dry-run`, **no borrar es el
comportamiento** y borrar es `--force`. Un `--dry-run` que hay que acordarse de escribir protege a
quien ya tiene cuidado. Y apareció un límite que no estaba previsto: **los volúmenes no llevan
etiquetas `hm.*`**, sólo las de Compose, así que los de un proyecto sin contenedores no se pueden
atribuir y no se tocan ni con `--force`; se listan aparte para que decida una persona. Nunca se
ejecuta un `prune` de Docker, y hay un test que lo comprueba. En la 1.6.0; documentado en
[docs/clean.md](../clean.md).
**Anotado para el futuro**: etiquetar también los volúmenes en la plantilla permitiría atribuirlos,
pero sólo a los creados después, así que no resuelve el caso que hoy duele.

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

### Área Release

Nace de la necesidad de validar versiones candidatas en proyectos reales y de compartirlas
entre compañeros. Detalle en el change [version-switching](../../openspec/changes/version-switching/).

#### REL-01 · `hm --version` con la referencia exacta
**Esfuerzo**: S. Usa `git describe --abbrev=0`, así que con once commits por encima del último
tag seguía diciendo `1.4.5`. Quien reporta un fallo no puede decir sobre qué lo reporta.

#### REL-02 · `hm switch`
**Esfuerzo**: M. **Depende de**: REL-01. Cambiar de versión y volver, con `--list` y
`--stable`, negándose si hay cambios sin guardar en la instalación.

#### REL-03 · `hm update` no debe sacar de un tag en silencio
**Esfuerzo**: S. **Es el urgente.** En un checkout desacoplado, `git pull origin HEAD` no
falla: trae la rama por defecto del remoto. Quien esté validando una candidata la pierde en
su primer `hm update`, sin ningún aviso.

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

#### TUI-01 · Dashboard de terminal (`hm tui`) — hecho
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
**Cómo quedó**: implementado como `hm tui`, no como `hm` sin argumentos: `hm` a secas sigue
mostrando la ayuda, que es lo que espera quien llega por primera vez y lo que asumen los
scripts. Sin `fzf` ni ninguna otra dependencia opcional: la navegación con teclas hacía
falta de todas formas, y tener dos caminos según lo que hubiera instalado significaba dos
comportamientos que documentar y mantener. Documentado en [docs/tui.md](../tui.md);
implementado en [terminal-dashboard](../../openspec/changes/archive/). La primera versión
parpadeaba y respondía en 404 ms por tecla; el render se rehízo en
[smooth-dashboard-rendering](../../openspec/changes/archive/) hasta 1,7 ms por fotograma, y de
ahí salió la comparativa de [tui-landscape.md](tui-landscape.md).

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
**Cómo quedó**: **opcional por proyecto** (`USE_PROXY`), así que nadie migra y la versión es 1.7.0
y no 2.0.0. Los proyectos con proxy **no publican ningún puerto**. Lo HTTP va por dominio y
subdominios; **MySQL y AMQP no se pueden enrutar por nombre** —Traefik lo rechaza: *has HostSNI
matcher, but no TLS on router*— y se alcanzan con `hm tunnel`. Hitch desaparece: sólo estaba para
dar HTTPS a Varnish. La superposición usa `!reset` de Compose 2.24, así que **la plantilla base no
se duplica**. Incluye NET-02: certificado comodín, obligado por los subdominios. Verificado
levantando dos proyectos a la vez. En la 1.7.0; documentado en [docs/proxy.md](../proxy.md).


#### NET-02 · Certificado wildcard
**Esfuerzo**: S. **Depende de**: NET-01. Un `mkcert` para `*.<dominio>` en lugar de uno por
proyecto, como hace `warden sign-certificate`.

#### NET-03 · dnsmasq
**Esfuerzo**: M. **Depende de**: NET-01. Resolver `*.local` (o `*.test`) sin escribir en
`/etc/hosts` con `sudo`, que es lo que hace hoy `set-host.sh`. **Ojo**: en macOS requiere
un resolvedor en `/etc/resolver/`, y `.local` colisiona con mDNS/Bonjour — **valorar
cambiar a `.test`**, que es el TLD reservado que usan Warden y DDEV.
**Decidido (26/08/2026)**: el TLD lo elige **cada proyecto**, con `.local` por defecto. Cambiarlo
por decreto obligaría a regenerar dominios y certificados en todos los proyectos existentes; así
nadie se mueve y quien quiera prueba `.test` sin arrastrar al resto.
**Cómo quedó (parcial)**: lo que da el valor es un `if` — comprobar si el dominio **ya resuelve**
antes de escribir en `/etc/hosts`. Hecho, y funciona con cualquier resolvedor, incluido el que ya
tenía la máquina. **Montar un dnsmasq propio se dejó fuera a sabiendas**: el puerto 53 ya estaba
ocupado por otro resolvedor, así que no había dónde probarlo, y construir a ciegas algo que pide
`sudo` y puede disputar un puerto no compensa. Documentado cómo conseguirlo en
[docs/dns.md](../dns.md). En la 1.7.0.

#### NET-04 · `hm share` con Cloudflared — hecho
**Origen**: cantera §3.5. **Decisión D3**. **Esfuerzo**: S.
**Casos de uso**: enseñar avances a cliente o QA sin desplegar, y **recibir webhooks
reales** de pasarelas de pago, ERPs y marketplaces contra el entorno local.
**Aceptación**: `hm share` devuelve una URL pública; avisa de que el entorno queda expuesto
y exige confirmación; `hm share --stop`; documenta que la URL cambia en cada arranque salvo
que se configure un túnel con nombre.
**Cómo quedó**: túneles rápidos, sin cuenta ni credenciales y sin instalar nada. **Verificado desde
la red de la empresa**, que era la única duda real: una primera medición dijo que estaba filtrado y
era falsa — había consultado la URL antes de que el túnel registrara sus conexiones con el borde.
No depende del proxy: el túnel se une a la red del proyecto. La confirmación nombra lo que se
expone, porque es la única de esta herramienta que protege algo que no está en la máquina. En la
1.7.0; documentado en [docs/share.md](../share.md).

### Área Entorno

#### ENV-01 · Nombre de proyecto derivado del directorio — hecho
**Origen**: cantera §3.12. **Esfuerzo**: S. DDEV lo ofrece como opción global precisamente
para que **cada worktree sea un proyecto distinto sin configurar nada**. Requiere decidir
qué pasa con el `COMPOSE_PROJECT_NAME` que hoy está versionado en `properties.json`.
**Cómo quedó**: el nombre configurado gana siempre, así que ningún proyecto existente se mueve —
era la restricción que mandaba, porque renombrar un entorno deja sus volúmenes atrás. Sólo se
deriva cuando no hay nombre, con la regla exacta de Compose, medida y vigilada por un test contra
`docker compose config`. `hm setup` deja de escribir la propiedad cuando coincide con el
directorio, que es lo que hacía que un clon heredara la identidad del original. Un worktree
resuelve al checkout principal, así que sigue sin tener entorno propio: eso es WT-02. En la
1.6.0; documentado en [docs/project-name.md](../project-name.md).

#### ENV-02 · Etiquetas `hm.*` en la plantilla compose
**Origen**: torre §5.2. **Esfuerzo**: S.
`hm.project`, `hm.worktree`, `hm.branch`, `hm.agent`, `hm.profile`, `hm.magento`.
Es la base de `list`, `clean`, el TUI, el dashboard y la detección de huérfanos. **Sin
estado que se desincronice**: nada de fichero de registro.

#### ENV-03 · Mailpit en lugar de Mailhog — hecho
**Esfuerzo**: S. **Mailhog está sin mantenimiento**; Warden migró a Mailpit en v0.15 y
nuestra plantilla sigue con Mailhog. Mantener compatibilidad de puerto/UI y actualizar
`requirements.json` y la documentación.
**Cómo quedó**: no como sustitución sino como **elección**, que es lo que permite que ningún
entorno en marcha tenga que moverse. Mailhog sigue siendo el valor por defecto; Mailpit se elige
en la instalación o cambiando una propiedad y regenerando. Los dos responden al nombre `mailhog`
en la red, así que un Magento ya instalado sigue entregando correo sin reconfigurarse — el alias
está comprobado levantando una red de verdad. La imagen se publica a mano y `hm doctor` avisa si
no está. En la 1.6.0; documentado en [docs/mail.md](../mail.md).
**Pendiente fuera de la herramienta**: publicar `hiberusmagento/mailpit` en Docker Hub.

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

#### DB-01 · `hm db snapshot` / `restore` / `list` — hecho
**Origen**: cantera §3.3. **Esfuerzo**: M.
**Propuesta**: snapshots con **backup en caliente** (`mariadb-backup`) dentro del
contenedor, con nombre y fecha, al estilo de `ddev snapshot`.
**Aceptación**: crear y restaurar sin parar el proyecto; `--name`; lista con tamaño y
origen; funciona con las versiones de MariaDB de `requirements.json`.
**Cómo quedó**: con volcado **lógico**, no con `mariadb-backup` como proponía esta ficha. La copia
física se crea en caliente pero **restaurarla exige parar el servidor** —hay que reemplazar su
directorio de datos—, y el criterio de aceptación de aquí al lado pedía restaurar sin parar el
proyecto. La lógica es más lenta y sirve entre versiones distintas de MariaDB. Las copias viven en
`~/.hm/snapshots/<proyecto>/`, fuera del proyecto, y sobreviven a `down -v`. Restaurar vacía el
esquema antes de cargar, para que devuelva la copia y no una mezcla. En la 1.6.0; documentado en
[docs/db.md](../db.md).
**Añadido a petición**: `hm db clear` / `--all` para vaciar copias, siempre preguntando y
enumerando lo que se destruye. Y la compatibilidad con **todas** las imágenes de base de datos
—de MariaDB 10.2 a 12.3— está verificada por un test de matriz, no supuesta.

#### DB-02 · Volumen "golden" y clonado — hecho
**Origen**: worktrees §4.2. **Esfuerzo**: M. **Depende de**: DB-01.
Clonar el volumen de datos desde una plantilla congelada para levantar entornos aislados en
segundos sin tocar el principal. Medido: `dbdata` 245 MB y `workspace` 695 MB en un
proyecto real, o sea clones de segundos.
**Cómo quedó**: `hm db freeze` congela el directorio de datos en un volumen etiquetado y
`hm db clone` lo copia sobre el del proyecto actual. Tres decisiones: la copia se hace con la
propia imagen del proyecto (ya está en la máquina y su `cp -a` reproduce un directorio de datos
sin discutir), el servidor se para mientras copia (InnoDB tiene páginas en memoria que aún no
están en los ficheros, así que copiar por debajo da un *crash* que recuperar, no una copia), y la
plantilla guarda la imagen con la que se hizo y clonar rechaza otra versión. La dirección es
`<proyecto>/<nombre>` porque el entorno que necesita una base de datos casi nunca es el que la
congeló — que es justo lo que necesita WT-02. En la 1.7.0; documentado en
[docs/db.md](../db.md).

#### DB-03 · Ciclo de vida seguro — hecho
**Esfuerzo**: S. `hm stop --snapshot`, confirmación en `down -v`, y acotar
`docker-stop-all` al proyecto o exigir confirmación explícita.
**Cómo quedó**: `down -v` enumera los volúmenes y ofrece **tres** respuestas, con «guardar y
destruir» por defecto — la que nadie lamenta. `docker-stop-all` no se acota al proyecto: se le pide
confirmación diciendo cuántos contenedores son ajenos, porque acotarlo cambiaría lo que hace un
comando que la gente ya usa a propósito. Nada pregunta sin terminal ni con `--yes`. En la 1.6.0;
documentado en [docs/down.md](../down.md) y [docs/stop.md](../stop.md).

#### DB-04 · Anonimización por defecto en entornos de agente
**Esfuerzo**: S. Apoyado en `hm masquerade`, que **ya existe**. Con agentes, la
anonimización deja de ser buena práctica y pasa a ser cumplimiento.

### Área Instalación

#### INST-01 · Admin aleatorio y 2FA con QR — hecho
**Origen**: cantera §3.9. **Esfuerzo**: S.
Hoy el admin sale de `data/config.json` con contraseña fija `Hiberus123` y sin resolver el
2FA, que es el primer tropiezo de cualquier instalación moderna. Warden genera contraseña
aleatoria y pinta el QR de Google Authenticator en el terminal.
**Cómo quedó**: contraseña de 20 alfanuméricos generada en cada instalación y **no guardada en
ningún fichero** — `data/config.json` es común a todos los proyectos y guardaba las respuestas, así
que la contraseña del último proyecto instalado quedaba en claro y se heredaba en el siguiente. El
segundo factor se da de alta con `security:tfa:google:set-secret` (nombre leído del código del
módulo, no supuesto) y el QR lo pinta `endroid/qr-code`, que **ya viene en el vendor de cualquier
Magento**: cero dependencias nuevas, ni en el host ni en la imagen. Si el módulo está desactivado
—como lo está en uno de los proyectos de la máquina— se informa y no se activa nada. En la 1.6.0;
documentado en [docs/install.md](../install.md).

#### INST-02 · `--clean-install` / `--db-dump`
**Esfuerzo**: S. Alinear `hm setup`/`hm install` con las opciones del bootstrap de Warden.

#### INST-03 · Proveedores `hm pull` / `hm push`
**Esfuerzo**: M. Formalizar `transfer-db`, `transfer-media` y `cloud` como proveedores
declarativos (Adobe Commerce Cloud, SSH/rsync, S3), ejecutables sin interacción.

### Área IA

#### AI-01 · `hm verify [--changed] [--json]` — hecho
**Esfuerzo**: M. **Depende de**: CLI-01.
PHPCS con estándar Magento2, PHPStan/Rector, `test-unit`, compilación de DI y validación de
XML/XSD, con salida estructurada. Sirve como comando manual, como hook de cierre de tarea
de los agentes y en CI. **El cuello de botella con agentes no es generar, es verificar.**
**Cómo quedó**: **descubre lo que hay** en vez de exigir una lista fija — se midió que de catorce
proyectos de la máquina, diez tenían PHPUnit, seis PHPStan, cinco el estándar de Magento y tres
nada. Lo ausente se informa como omitido, nunca como fallo. La sintaxis se comprueba siempre,
porque `php -l` no necesita nada instalado. Las lentas (pruebas y DI) sólo con `--all`, porque un
comando de cinco minutos no se ejecuta. No corrige nada a propósito. En la 1.7.0; documentado en
[docs/verify.md](../verify.md).

#### AI-02 · Permisos y guardarraíles generados — hecho
**Esfuerzo**: S. Clasificar cada comando como *seguro sin supervisión* o *requiere
confirmación* y generar desde ahí la configuración de permisos de cada plataforma. Hoy cada
persona del equipo mantiene esa lista a mano y todas distintas.
**Cómo quedó**: el riesgo se declara en `command_descriptions.json`, el fichero por el que hay que
pasar obligatoriamente al añadir un comando. Tres niveles, no dos: con dos habría que tratar
`hm start` como `hm down -v` —y entonces nadie lee las preguntas— o como `hm describe`, y entonces
no protege de nada. Dos reglas que aparecieron al clasificar: un comando que envuelve a otros se
clasifica por el peor (`hm db` restaura además de copiar), y uno que ejecuta lo que se le dé es
peligroso por inocente que sea su uso normal — `hm exec` y `hm bash` son puertas abiertas
disfrazadas. Las dos listas internas que ya existían quedan vigiladas por tests para que no
diverjan. En la 1.7.0; documentado en [docs/permissions.md](../permissions.md).

#### AI-03 · `hm mcp` (sólo lectura) — hecho  ·  #### AI-04 · `hm mcp` (escritura acotada)
**Esfuerzo**: M cada uno. Herramientas tipadas: lectura (describe, list, logs, `db.query`
sólo SELECT, estado de índices) y escritura acotada (cache clean/flush, reindex,
config:set). Las peligrosas (setup:upgrade, composer update, import de BD, `down -v`)
quedan fuera o exigen confirmación humana.
**Cómo quedó AI-03**: servidor en bash sobre stdin/stdout —el transporte es JSON-RPC delimitado
por saltos de línea, así que no hay framing que equivocar ni concurrencia que gestionar— con cinco
herramientas que envuelven comandos que ya responden en JSON, de modo que no aparece una segunda
fuente de verdad sobre el entorno. Dos decisiones: por stdout no sale nada que no sea protocolo
(cada comando envuelto va con stderr redirigido y stdin cerrado, porque un aviso suelto es un
error de parseo en el cliente y un servidor que "dejó de funcionar"), y una herramienta que falla
responde con un resultado marcado como error, no con un error de JSON-RPC, que la mayoría de
clientes muestran como servidor roto. `database_query` quita los comentarios antes de validar,
porque el comentario es donde se esconde la segunda sentencia. En la 1.7.0; documentado en
[docs/mcp.md](../mcp.md).

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

#### WT-02 · `hm worktree` con perfiles de entorno — hecho
**Esfuerzo**: L. **Depende de**: NET-01, DB-02. Perfiles `lite` (sólo PHP), `agent`
(php+nginx+db+search+redis) y `full`.
**Cómo quedó**: el registro vive en `~/.hm/worktrees/`, nunca en el checkout, porque
`properties.json` está versionado y el nombre de proyecto del worktree escrito ahí viajaría en el
commit de alguien. Ese fichero es además el interruptor: un worktree registrado se resuelve contra
sí mismo (su proyecto, sus volúmenes, sus montajes) y uno sin registrar conserva intactos los
rechazos de WT-01, que es el caso que sigue destruyendo datos. El perfil se expresa quitando
servicios (`!reset null`), no listando los que arrancar, así que `describe`, `doctor` y el panel
ven la verdad sin enterarse de que existen los perfiles. Las dependencias no se reinstalan: enlace
en Linux, copia del volumen de código en macOS. En la 1.7.0; documentado en
[docs/worktree.md](../worktree.md).

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
