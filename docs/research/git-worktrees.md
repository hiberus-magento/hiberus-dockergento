# Investigación: soporte de Git Worktrees en Dockergento

> Estado: **investigación / propuesta**. No hay código implementado todavía.
> Objetivo: permitir que varios agentes AI trabajen en paralelo, cada uno en su
> `git worktree`, sobre un mismo proyecto Magento gestionado con `hm`.

---

## 1. Resumen ejecutivo

Hoy `hm` **no es utilizable de forma segura desde un worktree**, y el problema no es
"falta de visibilidad de los contenedores" (eso funciona mejor de lo esperado), sino
tres cosas distintas que conviene no mezclar:

| Problema | Gravedad | Causa raíz |
|---|---|---|
| `hm` no arranca fuera del checkout principal si faltan los ficheros compose | Media | Todas las rutas se resuelven contra `$PWD` (`bin/run`) |
| **Ejecutar `hm start` / `rebuild` / `down` desde un worktree secuestra o destruye el stack principal** | **Alta** | Compose identifica el proyecto por *nombre*, no por ruta; los binds se re-apuntan al worktree |
| El código que ve el contenedor es siempre el del checkout principal | Alta | Los bind mounts se generaron apuntando a `{MAGENTO_DIR}` del checkout principal |

Conclusión: **compartir contenedores tal cual sirve para muy poco**, porque compartir
contenedores implica compartir el código montado. Lo que hace falta es un modelo
intermedio: **servicios pesados compartidos (db, search, redis, rabbitmq, mailhog) +
capa de aplicación propia por worktree (phpfpm + nginx) + aislamiento lógico de datos**.

Recomendación en 3 fases:

1. **Fase 1 — `attach` seguro** (bajo coste): que `hm` funcione desde un worktree
   apuntando al stack principal, con guardarraíles que impidan `up`/`down`/`rebuild`.
   Resuelve ya el 80 % de las operaciones de sólo lectura de un agente (`mysql -q`,
   `magento cache:clean`, logs, inspección).
2. **Fase 2 — stack ligero por worktree** (el objetivo real): `phpfpm` + `nginx`
   propios montando el código del worktree, conectados a los servicios de datos del
   stack principal a través de una red compartida, con esquema de BD / prefijo de
   índice / vhost propios.
3. **Fase 3 — proxy compartido + ciclo de vida**: un único reverse proxy en 80/443
   que enruta por `Host`, más `hm worktree list|down|gc` para no dejar basura.

---

## 2. Diagnóstico del estado actual (con evidencias)

### 2.1 Todo se resuelve contra `$PWD`

`bin/run`:

```bash
export CUSTOM_COMMANDS_DIR="$PWD/config/$COMMAND_BIN_NAME/commands"
export CUSTOM_PROPERTIES_DIR="$PWD/$DOCKER_CONFIG_DIR"      # $PWD/config/docker
export DOCKER_COMPOSE="$compose_cmd -f $DOCKER_COMPOSE_FILE -f $DOCKER_COMPOSE_FILE_MACHINE"
```

`DOCKER_COMPOSE_FILE` es `docker-compose.yml` **relativo**, así que el contexto de
compose es siempre el directorio de trabajo actual.

Comprobado: ejecutar `hm bash` fuera de un proyecto da

```
open .../docker-compose.yml: no such file or directory
Docker is not properly configured or docker is not running. Please execute: hm setup
```

### 2.2 Pero en un worktree los ficheros SÍ están (y eso es peor)

En un proyecto real (`sports-emotion`) están versionados en git:

```
config/docker/properties.json
docker-compose.yml
docker-compose.dev.mac.yml
docker-compose.dev.linux.yml
```

Es decir, el worktree hereda **el mismo `COMPOSE_PROJECT_NAME`** y unos ficheros compose
válidos. `hm` arranca sin quejarse... y opera sobre el proyecto del checkout principal.

### 2.3 Lo bueno: Compose resuelve por nombre de proyecto, no por ruta

Verificado en la máquina, desde un directorio sin ningún fichero compose:

```bash
docker compose -p sports-emotion ps      # lista los 9 servicios
docker compose -p sports-emotion exec -T phpfpm sh -c 'pwd'   # → /var/www/html
```

**Esto es la base técnica de la Fase 1**: para `exec`, `ps`, `logs`, `stop` no hacen
falta los ficheros `-f`, basta `-p <nombre>`. Un worktree puede "ver" los contenedores
del checkout principal sin copiar nada.

### 2.4 Lo malo: `up` desde otro directorio secuestra el stack

Laboratorio reproducible (dos directorios `A` y `B` con el mismo nombre de proyecto):

```
mount después de `up` desde A: .../A/src
mount después de `up` desde B: .../B/src     # mismo contenedor, RECREADO
```

Compose no crea un segundo stack: **recrea el contenedor existente con los binds del
directorio desde el que se invoca**. Traducido a nuestro caso: un agente que ejecute
`hm start` o `hm rebuild` desde su worktree **re-apunta silenciosamente el entorno
principal a su worktree**, y `hm down -v` lo destruye junto con sus volúmenes (BD
incluida). Es el riesgo más serio de la situación actual.

### 2.5 El código montado es el del checkout principal

Mounts reales de `sports-emotion-phpfpm-1` (macOS):

```
volume sports-emotion_workspace          -> /var/www/html      (vendor, generated, pub/static…)
bind  .../commerce/app                   -> /var/www/html/app
bind  .../commerce/config                -> /var/www/html/config
bind  .../commerce/composer.json|.lock   -> …
bind  .../commerce/.git                  -> /var/www/html/.git
bind  .../commerce/tests, .github, …
```

Aunque el agente "vea" los contenedores desde su worktree, `hm magento`, `hm composer`
o los tests se ejecutan sobre el código de `commerce/`, no sobre el suyo. **Compartir
contenedores sin más no habilita el trabajo en paralelo.**

### 2.6 Detalles que romperán una implementación ingenua

- **Puertos fijos** en `docker-compose.template.yml`: `80`, `443`, `3306`, `9200`,
  `8025`, `5672`, `15672`. Dos stacks completos simultáneos son imposibles sin
  parametrizar puertos o sin proxy.
- **`.git` en un worktree es un fichero**, no un directorio
  (`gitdir: /Users/…/hm/.git/worktrees/wt-test`). El bind `{MAGENTO_DIR}/.git` de la
  plantilla de mac dejaría git roto dentro del contenedor salvo que se monte también
  el *common dir*. En la práctica: en stacks de worktree conviene **no montar `.git`**.
- **`hitch` → `varnish` → `nginx`** están cableados por nombre DNS
  (`--backend=[varnish]:6081`, `.host = "nginx"` en el VCL). Si un stack de worktree se
  une a la red del principal con servicios homónimos, los alias DNS `nginx`/`db`
  quedan **ambiguos** (round-robin entre proyectos). Hay que unir a la red compartida
  **sólo** lo que necesita datos (phpfpm), o usar una red dedicada.
- **`nginx` ↔ `phpfpm` hablan por socket UNIX** (`sockdata:/sock`), no por TCP: cada
  stack de worktree necesita su propio volumen `sockdata` (lo tendrá por proyecto).
- **`hm docker-stop-all`** para *todos* los contenedores de la máquina: veneno en un
  entorno multi-agente. Debe quedar prohibido o acotado al proyecto.
- **Higiene de recursos**: la máquina ya tiene **152 volúmenes / 69 GB**. Un flujo con
  worktrees efímeros sin GC lo multiplica.

---

## 3. Modelos de arquitectura evaluados

| | **A. Attach (compartir todo)** | **B. Híbrido (recomendado)** | **C. Stack completo por worktree** |
|---|---|---|---|
| phpfpm/nginx | compartidos | **propios por worktree** | propios |
| db/search/redis/rabbit | compartidos | **compartidos** (esquema/prefijo propio) | propios |
| Código del worktree visible | ❌ | ✅ | ✅ |
| Puertos | sin conflicto | sin conflicto (proxy o puerto efímero) | conflicto: hay que parametrizar todo |
| RAM extra por agente | 0 | ~300–600 MB | ~3–5 GB |
| Tiempo de alta | inmediato | segundos–minutos (clonando volúmenes) | 20–60 min (`composer install` + import) |
| Aislamiento de datos | ninguno | lógico (esquema/prefijo/vhost) o volumen clonado | total |
| Esfuerzo de implementación | bajo | medio | alto |

**A** sólo sirve para operaciones de lectura/diagnóstico. **C** no escala en Docker
Desktop (memoria y tiempo). **B** es el punto óptimo y es el que propongo desarrollar.

---

## 4. Aislamiento de base de datos

El riesgo real que planteas (dos agentes pisándose con `setup:upgrade`, `config:set`,
reindexados, `cache:flush`) se resuelve por capas, de más barato a más aislado:

### 4.1 Esquema por worktree en el mismo contenedor (recomendado por defecto)

```sql
CREATE DATABASE magento_wt_<slug>;
```
copia con `mariadb-dump --single-transaction --quick magento | mariadb magento_wt_<slug>`,
y el worktree apunta ahí vía `app/etc/env.php` (`db/connection/default/dbname`).

- Sin RAM extra, sin puertos, sin contenedores nuevos.
- Coste = tamaño de la BD. Medido en `sports-emotion`: **`dbdata` = 245 MB** → clon en
  segundos. En clientes grandes (5–20 GB) sube a minutos: ahí conviene 4.2.

### 4.2 Clon de volumen + contenedor `db` propio

```bash
docker volume create <proj>_dbdata_wt_<slug>
docker run --rm -v <proj>_dbdata:/from -v <proj>_dbdata_wt_<slug>:/to alpine \
  sh -c 'cp -a /from/. /to/'
```

- Aislamiento total (incluye configuración del motor y `information_schema`).
- Requiere consistencia: parar el `db` principal un instante, o usar `mariadb-backup`.
- Coste: disco = tamaño BD por worktree; RAM ~300–500 MB por instancia.

**Variante recomendada: volumen "golden"**. `hm db snapshot` congela una copia
plantilla una sola vez; cada worktree clona **desde la plantilla**, sin tocar nunca el
stack principal y sin pararlo. Es rápido, repetible y da entornos deterministas para
los agentes.

### 4.3 El resto de servicios con estado, por namespace lógico

| Servicio | Mecanismo de aislamiento |
|---|---|
| OpenSearch / Elasticsearch | `catalog/search/*_index_prefix` distinto por worktree |
| Redis / Valkey | índice de base distinto en `env.php` (cache, page_cache, session) |
| RabbitMQ | vhost propio (`queue/amqp/virtualhost`) |
| Mailhog | compartible sin problema |
| `pub/media`, `pub/static`, `var/` | ya viven en el worktree (o en su volumen `workspace`) |

### 4.4 Aceleración del arranque en macOS

En mac el código "pesado" (vendor, generated, pub/static) está en el volumen nombrado
`workspace` (**695 MB medidos**). Clonarlo igual que el `dbdata` evita un
`composer install` completo por worktree: alta en segundos en lugar de decenas de minutos.

---

## 5. Superficie de comandos propuesta

```bash
hm worktree init [--mode=attach|isolated]   # registra el worktree y genera su config
hm worktree up                              # levanta phpfpm+nginx del worktree (modo isolated)
hm worktree status                          # a qué stack apunta este directorio y por qué
hm worktree list                            # todos los worktrees registrados del proyecto
hm worktree down [--volumes]                # baja SOLO lo del worktree
hm worktree gc                              # limpia stacks/volúmenes de worktrees ya borrados
hm worktree db clone [--from=snapshot]      # esquema o volumen propio
hm db snapshot [<nombre>]                   # volumen "golden" reutilizable
```

Variables de entorno de escape (útiles para agentes/CI):
`HM_PROJECT_DIR`, `HM_COMPOSE_PROJECT`, `HM_WORKTREE_MODE`.

---

## 6. Cambios concretos por fichero

### Fase 1 — attach seguro

| Fichero | Cambio |
|---|---|
| `console/helpers/worktree.sh` *(nuevo)* | `resolve_project_root()`: `git rev-parse --path-format=absolute --git-common-dir` → si acaba en `/.git`, el checkout principal es su directorio padre (**verificado**: desde un worktree devuelve `/Users/ddelgado/hm/.git`). Detecta worktree comparando con `$PWD`. |
| `bin/run` | Resolver `CUSTOM_PROPERTIES_DIR` y los `-f` contra el root resuelto, en rutas **absolutas**, y añadir `--project-directory <root>` + `-p <COMPOSE_PROJECT_NAME>`. |
| `bin/run` (`validate_command`) | Guardarraíl: en worktree y modo `attach`, bloquear `start`, `stop`, `restart`, `rebuild`, `down`, `setup`, `install`, `docker-stop-all` salvo `--force`, con mensaje explicativo. |
| `console/commands/docker-stop-all.sh` | Acotar al proyecto o exigir confirmación explícita. |
| `console/helpers/docker.sh` | `is_run_service()` ya filtra por `COMPOSE_PROJECT_NAME`: revisar que el nombre venga del root resuelto y no del `properties.json` del worktree. |

### Fase 2 — stack ligero por worktree

| Fichero | Cambio |
|---|---|
| `docker-compose/docker-compose.worktree.template.yml` *(nuevo)* | Sólo `phpfpm` + `nginx`; binds al worktree; **sin** bind de `.git`; sin puertos publicados; red `default` propia + red externa compartida. |
| `docker-compose/docker-compose.template.yml` | Añadir red `hm_shared` (external o creada por el proyecto) a `db`, `search`, `redis`, `rabbitmq`, `mailhog`, con alias explícitos. |
| `console/tasks/write_from_docker-compose_templates.sh` | Soportar la nueva plantilla y el sufijo de proyecto `<base>-wt-<slug>`. |
| `console/commands/worktree.sh` *(nuevo)* | Orquestación: slug determinista desde la rama (lowercase, `[a-z0-9_-]`), clonado de volúmenes, generación de `env.php` de override, alta/baja. |
| `data/command_descriptions.json` + `docs/worktree.md` | Documentación y ayuda del comando. |

### Fase 3 — proxy y DX

| Fichero | Cambio |
|---|---|
| Proxy compartido (Traefik/nginx) | Único contenedor en 80/443, certificado wildcard `mkcert *.<dominio>`, ruteo por `Host: wt-<slug>.<dominio>` → nginx del worktree. Elimina de raíz el conflicto de puertos y permite `base_url` estable por worktree. |
| `console/commands/set-host.sh`, `ssl.sh` | Soportar dominio derivado del worktree. |
| `console/tasks/set_magento_configs.sh` | Aplicar `base_url`, prefijo de índice, etc. del worktree. |

---

## 7. Plan de PoC (1–2 días, sin tocar el CLI)

Validar a mano, sobre `sports-emotion`, antes de escribir código:

1. `git worktree add ../se-wt-a feature/x`.
2. Crear a mano `docker-compose.wt.yml` con `phpfpm` + `nginx` apuntando al worktree,
   proyecto `sports-emotion-wt-a`, unido a la red `sports-emotion_default` como externa.
3. Clonar `sports-emotion_workspace` → `sports-emotion-wt-a_workspace`.
4. `mariadb-dump magento | mariadb magento_wt_a` en el `db` compartido.
5. `env.php` del worktree: `dbname=magento_wt_a`, redis db distinto, vhost distinto,
   `catalog/search/opensearch_index_prefix=wt_a`.
6. Publicar el nginx del worktree en un puerto efímero y `base_url` acorde.
7. **Criterios de aceptación**: (a) el sitio del worktree carga su propio código;
   (b) `bin/magento setup:upgrade` en el worktree no altera el principal;
   (c) reindexar en uno no rompe el buscador del otro; (d) `docker compose -p
   sports-emotion-wt-a down -v` no toca el stack principal.

---

## 8. Riesgos y preguntas abiertas

- **Colisión de alias DNS** al unir proyectos a una misma red (§2.6). Es el punto más
  probable de fallo del PoC.
- **Consistencia del clon de BD en caliente**: decidir entre parar el `db` un instante,
  `mariadb-backup`, o el enfoque de volumen "golden" (mi recomendación).
- **Rendimiento en macOS**: N stacks × binds `:cached` sobre virtiofs. Medir con 3
  agentes simultáneos antes de prometer números.
- **Ficheros compose versionados**: si `docker-compose.yml` está en git, cualquier
  regeneración desde un worktree ensucia el diff de la rama del agente. Habría que
  generar los ficheros del worktree en un directorio ignorado (`.hm/worktree/`).
- **`COMPOSE_PROJECT_NAME` en `properties.json` versionado**: al derivarlo por worktree
  hay que asegurarse de no reescribir el fichero del proyecto (evitar `save_properties`
  en modo worktree).
- **Licencias/consumo**: cada stack de worktree añade un `phpfpm`; con >4 agentes en
  paralelo conviene revisar el límite de memoria de Docker Desktop.

---

## 9. Siguiente paso recomendado

Implementar **Fase 1** (helper de resolución + guardarraíles + `hm worktree status`):
es pequeña, elimina hoy mismo el riesgo de destrucción del entorno principal y
desbloquea a los agentes para todo lo que sea diagnóstico. En paralelo, ejecutar el
PoC de §7 para confirmar la viabilidad de la Fase 2 antes de comprometer diseño.

---
---

# Parte II — Estado del arte y estrategia

> Añadido tras investigar cómo resuelve esto la industria y, en concreto, la comunidad
> de Adobe Commerce. **Esta parte corrige la recomendación de la Parte I.**

## 10. Qué hay realmente detrás de las "granjas de 40 agentes"

Tres cosas que no se ven en el tuit:

1. **En local, nadie va a 40.** La guía práctica de operación de flotas sitúa el punto
   dulce en **3–5 agentes en paralelo** por desarrollador, y los equipos que van más
   allá llegan a 4–8. Por encima de eso el cuello de botella deja de ser la generación
   y pasa a ser **la revisión humana**: más agentes sólo producen más cola de PRs.
2. **Los números grandes son cloud, no portátil.** Las cifras de decenas o cientos de
   agentes salen de sandboxes efímeras remotas (Firecracker, gVisor, Kata; Modal,
   Northflank y similares), donde cada agente arranca una microVM aislada en segundos y
   se destruye al terminar. No es "mi Mac con 40 Dockers".
3. **Casi siempre son stacks stateless.** En un proyecto Node o Python, "el entorno" es
   `npm install` y un puerto. El coste marginal de un agente más es cercano a cero.
   Magento es el caso opuesto: base de datos de varios GB, OpenSearch, Redis, Varnish,
   `setup:upgrade`, compilación de DI y despliegue de estáticos.

Dato de coste, para calibrar expectativas: operar 5–10 agentes en paralelo se mueve en
el orden de **50–130 $/día** en tokens. La escala tiene un precio que no es sólo RAM.

**Conclusión:** el objetivo razonable no es 40 agentes. Es **3–8 agentes por
desarrollador con entornos fiables**, y que la mayoría de ellos ni siquiera necesite un
Magento levantado (ver §12).

## 11. Cómo lo resuelve la comunidad Adobe Commerce

La respuesta corta: **el problema de "varios entornos Magento a la vez" está resuelto
desde hace años; el de "varios agentes" no lo ha planteado nadie.**

| Herramienta | Cómo resuelve el multi-entorno | ¿Worktrees? | ¿Agentes? |
|---|---|---|---|
| **DDEV** | Router compartido en 80/443, dominios `*.ddev.site`, nombre de proyecto derivado del directorio, `snapshot`/`import-db` con herramientas de backup en caliente | **Sí, guía oficial**: cada worktree es un proyecto con su URL y su BD importada de un snapshot | No |
| **Warden** | Servicios **globales compartidos** (Traefik, resolver DNS, CA de SSL, Portainer) + stack aislado por proyecto, ruteo por `Host` con labels de Traefik | No documentado, pero encaja: un worktree sería otro proyecto | No |
| **markshust/docker-magento** | Ruteo por hostname vía nginx; multi-proyecto a base de configuración manual | No | No |
| **Adobe Commerce Cloud** | Entornos de integración por rama — pero **limitados** (dos ramas de integración tras el cambio a Enhanced Integration Environments en Pro) | N/A | No |

Y en la capa de IA, la comunidad Magento está muy activa… pero **toda en la capa de
prompt**: colecciones de subagentes para Magento, skills de Claude Code para operar la
tienda, servidores MCP de Adobe Commerce. Los flujos publicados por agencias describen
**un agente por proyecto en sesiones secuenciales**; ninguno menciona worktrees,
aislamiento de base de datos ni concurrencia de entornos.

Dos consecuencias directas:

- **El patrón de infraestructura ya está inventado y probado**: *no compartas
  contenedores; comparte un router y abarata la creación del stack*. Es exactamente lo
  que hacen DDEV y Warden.
- **El hueco real está en el nivel de arriba**: nadie ha construido "entornos efímeros
  por agente" para Adobe Commerce. Ahí sí seríamos pioneros.

## 12. Corrección de la Parte I

La Parte I proponía como objetivo el **modelo híbrido** (compartir `db`, `search`,
`redis` y `rabbitmq` entre worktrees, con namespaces lógicos). Visto el estado del arte,
eso es **más exótico y más frágil** que lo que hace el resto del mundo: obliga a
namespacing dentro de Magento (esquema, prefijo de índice, índice de Redis, vhost),
tiene el problema de alias DNS ambiguos y deja acoplamientos difíciles de depurar.

**Recomendación revisada:** copiar el patrón DDEV/Warden — *stack propio por entorno,
router compartido, creación barata* — y hacerlo asequible con **niveles de entorno**,
que es lo que de verdad permite escalar en un proyecto tan pesado como Magento:

| Nivel | Qué levanta | Para qué | Coste | Cuántos a la vez |
|---|---|---|---|---|
| **0 · sin servicios** | sólo un contenedor PHP con el vendor montado en solo-lectura | lint, PHPCS, PHPStan/Rector, tests unitarios, generación y refactor de código | ~50 MB | decenas |
| **1 · stack de agente** | phpfpm + nginx + db + search + redis (sin varnish, hitch, rabbitmq ni mailhog) | verificar en navegador, `setup:upgrade`, tests de integración | ~1,5–2 GB | 3–6 |
| **2 · stack canónico** | el entorno completo actual | el entorno del desarrollador, QA final | ~3–5 GB | 1 |

La clave está en el nivel 0: **la mayor parte del trabajo de un agente en Magento no
necesita Magento levantado**. Medir ese porcentaje en nuestros propios flujos es el
primer experimento que hay que hacer (§14). Y para lo que sí lo necesita, la guía de
operación de flotas recomienda un patrón más simple que aislar: **serializar el recurso
caro con un lock** — un solo `setup:upgrade` o un solo reindex a la vez mientras todo lo
demás sigue en paralelo.

## 13. ¿Docker sí o no? ¿Y por qué no en el portátil?

- **Sin Docker no es la respuesta.** Un Magento nativo por worktree no es reproducible y
  multiplica el problema de versiones de PHP y extensiones. Docker no es el cañón: el
  cañón es *levantar nueve servicios por agente*.
- **Linux &gt; macOS para esto.** En Linux los bind mounts son nativos; en macOS todo pasa
  por virtiofs y N stacks concurrentes degradan mucho antes.
- **La escala real no la da el worktree, la da sacar el entorno del portátil.** 40
  entornos Magento no caben en un MacBook por RAM, y no hay arquitectura que lo arregle.
  En un servidor de desarrollo compartido (128 GB) sí caben del orden de 10–15 stacks de
  nivel 1. Si algún día queremos flotas grandes, el camino es **agentes en remoto**
  (dev-server o sandboxes efímeras), no más contenedores en local.

## 14. ¿Construimos herramienta? Sí, pero acotada

No un binario nuevo desde cero, y desde luego no otro DDEV. La propuesta:

1. **`hm worktree` / `hm env`** dentro de Dockergento, con los niveles de §12 como
   perfiles (`--profile=lite|agent|full`).
2. **Un `hm proxy` global** al estilo de los servicios globales de Warden: un Traefik en
   80/443 con certificado wildcard, para que N entornos convivan sin tocar puertos.
   Esto **también beneficia al equipo aunque no usemos agentes**: hoy no se pueden tener
   dos proyectos Dockergento arriba a la vez.
3. **`hm db snapshot` / `hm db clone`** al estilo de los snapshots de DDEV, con la
   variante de volumen "golden" de §4.
4. Los guardarraíles de la Fase 1 de la Parte I, que siguen siendo válidos y urgentes.

Dos avisos honestos antes de invertir:

- **La alternativa seria es adoptar Warden o DDEV** en lugar de mantener Dockergento.
  Ambos ya tienen el router, el multi-proyecto y los snapshots. Merece una decisión
  explícita y no por omisión: lo que ganamos manteniendo Dockergento son las imágenes
  propias, la integración con nuestros flujos y el control de versiones de Magento; lo
  que perdemos es todo lo que ya está hecho ahí fuera.
- **Lo verdaderamente diferencial no es el contenedor, es el ciclo**: snapshot de BD,
  alta y baja de entorno en segundos, y verificación automática antes del merge. Si sólo
  copiamos el router, habremos hecho un DDEV peor.

### Experimento que decide todo esto (una semana)

1. Instrumentar una semana de trabajo real con agentes y clasificar cada tarea: ¿necesitó
   Magento levantado, sí o no? Ese porcentaje decide si hace falta el nivel 1 o basta el 0.
2. Medir cuántos entornos de nivel 1 aguanta un portátil típico del equipo y uno de los
   servidores de desarrollo.
3. Cronometrar el alta de un entorno de nivel 1 desde snapshot. Si baja de ~2 minutos,
   el modelo es viable; si se va a 20, hay que rediseñar el snapshot antes de seguir.
