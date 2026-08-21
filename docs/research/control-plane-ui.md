# Investigación: dashboard y plano de control de Dockergento

> Estado: **investigación / propuesta de arquitectura**. Nada implementado.
> Relacionado: [git-worktrees.md](git-worktrees.md) (entornos por agente),
> [ddev-warden-feature-mining.md](ddev-warden-feature-mining.md) (`hm proxy`, `describe`,
> snapshots) y [ai-features.md](ai-features.md) (contrato JSON).

---

## 1. La idea

Una vez exista el **proxy global** (`hm proxy up`), la máquina deja de tener "un proyecto
levantado" y pasa a tener **una flota**: N proyectos, cada uno con M worktrees, algunos
operados por agentes, con sus snapshots de base de datos y sus recursos. Eso ya no cabe en
la cabeza de nadie ni en la salida de `docker ps`.

La propuesta es que el propio proxy sirva un **plano de control de Dockergento**: una web
local en, por ejemplo, `https://hm.local/` que muestre qué hay, en qué estado, y permita
las acciones seguras — sin quitar nada de la CLI, que sigue siendo la interfaz completa.

## 2. Precedentes: qué ha funcionado y qué no

| Caso | Qué es | Estado |
|---|---|---|
| **`ddev` sin argumentos** | Un **dashboard de terminal (TUI)** con todos los proyectos, su estado y atajos de teclado | ✅ Oficial, mantenido, es la respuesta por defecto de DDEV |
| **`ddev/ddev-ui`** | GUI de escritorio oficial (Electron + React) | ❌ **Abandonada** ("not currently maintained") |
| **DDEV Manager, DDEVBar, DDEVUI, extensión de VS Code** | Wrappers de terceros: escritorio, barra de menús, editor | Vivos, pero fuera del core |
| **Warden** | Delega en **Portainer** como servicio global | Vivo, pero es un visor genérico de Docker |
| **Traefik** | Trae su propio dashboard de routers y servicios | Vivo, útil, pero habla de *routers*, no de proyectos |

**Tres lecciones, y son incómodas:**

1. La GUI oficial de DDEV **murió**, mientras que el TUI que viaja dentro del binario
   sigue vivo. El coste de mantener una app gráfica separada del CLI es real.
2. Lo que sobrevive fuera del core son integraciones con el editor y apps nativas hechas
   por terceros que asumen ese coste.
3. Warden resolvió la papeleta **sin construir nada**: enchufó Portainer.

De ahí el criterio que propongo: *cualquier plano de control debe viajar con la CLI,
versionarse con ella y no exigir una cadena de build aparte.*

## 3. El principio que lo ordena todo: un contrato, tres consumidores

El dashboard **no es una funcionalidad nueva**: es el tercer consumidor de la pieza que ya
está priorizada en las otras dos investigaciones.

```
                     hm describe --json
                     hm list --json          ← el contrato
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   CLI humana          Dashboard web        Agentes (MCP)
   (hoy)               (esta propuesta)     (ai-features.md)
```

Esto tiene dos consecuencias prácticas:

- **Construir el contrato JSON aporta valor tres veces.** Si sólo se hiciera el dashboard,
  seguiría siendo la misma inversión.
- **El dashboard no puede hacer nada que la CLI no exponga.** Ninguna lógica de negocio
  vive en la web; es una vista.

## 4. Qué NO es: no es un Portainer

Portainer y Docker Desktop ya muestran contenedores, imágenes y volúmenes. Repetir eso no
aporta nada y, además, ya dijimos que no queríamos mantenerlo
([feature mining](ddev-warden-feature-mining.md) §5).

La diferencia está en el **vocabulario**. Portainer habla de contenedores; nosotros
hablamos de:

| Portainer ve | Dockergento debe ver |
|---|---|
| 9 contenedores con nombres largos | **1 proyecto** con su URL, su versión de Magento y su estado |
| `proj-wt-a-phpfpm-1` | **1 worktree**, su rama, su tarea y el agente que lo ocupa |
| un volumen de 245 MB | **1 snapshot** de BD, con fecha y de qué entorno salió |
| contenedor "unhealthy" | **`hm doctor`**: puerto ocupado, certificado caducado, índice roto |

Esa traducción de dominio es todo el valor. Si la web no habla de proyectos, worktrees,
agentes y snapshots, no merece existir.

## 5. Arquitectura propuesta

### 5.1 Dónde vive

Un servicio más del stack global, junto a Traefik y dnsmasq:

```
hm proxy up
  ├── traefik          80/443, enruta *.local por Host
  ├── dnsmasq          resuelve *.local sin tocar /etc/hosts
  └── hm-dashboard     https://hm.local/   ← plano de control
```

Se distribuye como imagen (`hiberusmagento/dashboard:<versión>`), que encaja con el
pipeline que ya existe (`.github/workflows/dockerhub-publish.yml`). Se versiona junto a la
CLI: la web nunca va por delante de los comandos que llama.

### 5.2 Cómo descubre lo que hay: etiquetas, no registro

Docker Compose ya etiqueta todo por proyecto, y Traefik funciona así. Añadimos etiquetas
propias en la plantilla:

```yaml
labels:
  hm.project: example-shop
  hm.worktree: feature-x        # vacío en el checkout principal
  hm.branch: feature/x
  hm.agent: claude-3            # opcional, lo estampa el flujo de agentes
  hm.profile: agent             # lite | agent | full
  hm.magento: "2.4.9"
```

Ventajas frente a un fichero de registro: no hay estado que se desincronice, sobrevive a
`docker compose up` desde cualquier sitio y lo puede leer también la CLI (`hm list`).
Para la actualización en vivo, `docker events` en lugar de *polling*.

### 5.3 La frontera: qué puede hacer la web y qué no

Esta es la decisión de diseño más importante:

> **Lo que se puede hacer a través del socket de Docker, lo hace el dashboard.
> Lo que toca el sistema de ficheros del host, lo hace la CLI.**

| Acción | Dashboard | Por qué |
|---|---|---|
| start / stop / restart de un proyecto o servicio | ✅ | API de Docker |
| ver logs, seguir logs | ✅ | API de Docker |
| abrir la URL, copiar credenciales | ✅ | Sólo lectura |
| `cache:clean`, reindex, ver estado de índices | ✅ | `exec` en el contenedor |
| snapshot y restore de BD | ✅ | `exec` + volúmenes |
| destruir un worktree o un entorno | ⚠️ con confirmación | API de Docker, pero destructivo |
| `hm setup`, `create-project`, `composer install` en Mac | ❌ CLI | Necesitan el FS del host |
| crear un worktree (`git worktree add`) | ❌ CLI | Necesita el FS del host y git |

Para lo que queda fuera, la web **muestra el comando exacto listo para copiar**. Es
honesto, enseña la CLI y evita el error de intentar meter un demonio en el host — que es
justo la pieza que dispara el coste de mantenimiento.

### 5.4 Stack técnico recomendado

**PHP + htmx/Alpine, sin cadena de build.** Argumentos, en este orden:

1. **Lo mantiene el departamento.** Somos un equipo de Magento: PHP y Alpine (que ya usa
   Hyvä) son el terreno conocido. Una SPA en React sería la vía rápida al mismo destino
   que `ddev-ui`.
2. **Sin `npm run build`.** Nada de artefactos compilados en el repositorio ni de una
   cadena de herramientas que se pudre.
3. **La imagen base ya existe**: nuestras propias imágenes de PHP.

Alternativa razonable: un binario Go embebiendo los estáticos (es lo que hacen DDEV y
Warden). Más eficiente y autocontenido, pero fuera del stack del equipo. **Decisión abierta.**

## 6. Pantallas

1. **Flota** — tarjetas por proyecto: estado, URL, versión de Magento, worktrees activos,
   RAM y disco. Avisos globales del `doctor` arriba.
2. **Proyecto** — servicios y su salud, URLs, credenciales (ocultas por defecto), versiones,
   Xdebug on/off, accesos directos a Mailpit, RabbitMQ y OpenSearch.
3. **Worktrees** — la vista que hace legible el trabajo multi-agente: rama, tarea, agente,
   perfil, estado, última actividad, y acciones de abrir, parar, destruir o clonar BD.
   Con detección de **huérfanos**: contenedores cuyo worktree ya no existe en git.
4. **Snapshots** — lista por proyecto con fecha, tamaño y origen; crear y restaurar.
5. **Doctor** — puertos ocupados y por quién, certificados, DNS, espacio de volúmenes,
   servicios caídos. Es `hm doctor` renderizado.
6. **Actividad** — qué comandos se han ejecutado desde la web, con quién y cuándo.

## 7. Seguridad: es una web local con permisos de administrador

No es paranoia: es el modo en que estas herramientas se rompen.

- **Validar la cabecera `Host`.** Un dominio local no protege de un ataque de *DNS
  rebinding*: el navegador de la víctima hace las peticiones desde dentro del límite de
  mismo origen. Aceptar sólo `hm.local` y rechazar el resto.
- **Tokens CSRF también en local.** El *cookie* se envía igual desde una pestaña
  maliciosa; que la aplicación sea local no cambia nada.
- **El socket de Docker no se expone jamás al navegador.** Lo usa el backend; la API HTTP
  ofrece verbos de dominio (`POST /projects/x/restart`), nunca `docker` en crudo.
- **Escritura desactivada por defecto.** El dashboard arranca en sólo lectura; las acciones
  se habilitan de forma explícita (`hm proxy up --with-actions`).
- **Nada de puertos publicados en `0.0.0.0`.** Sólo a través de Traefik y con escucha en
  loopback.
- **Las acciones destructivas piden confirmación escrita** y quedan registradas en la vista
  de actividad.

## 8. Fases

| Fase | Qué entra | Coste | Valor |
|---|---|---|---|
| **0** | `hm describe --json`, `hm list --json` y etiquetas `hm.*` en la plantilla | Bajo | Ya aporta a CLI y agentes, sin web |
| **1** | Dashboard **sólo lectura**: flota, proyecto, doctor | Medio | Hace visible la flota; riesgo casi nulo |
| **2** | Acciones seguras: start/stop/restart, logs, abrir, cache/reindex | Medio | Sustituye el 80 % de los comandos del día a día |
| **3** | Vista de worktrees y agentes, con recolección de huérfanos | Medio | Es **el** plano de control multi-agente |
| **4** | Snapshots desde la web y acciones destructivas con confirmación | Medio | Cierra el ciclo de pruebas y migraciones |

La Fase 0 es la única que hay que hacer sí o sí: es común a las tres investigaciones. Todo
lo demás se puede parar en cualquier punto sin dejar nada a medias.

## 9. Riesgos y decisiones abiertas

- **Es el primer trozo de Dockergento que no es bash.** Cambia el perfil de quien lo
  mantiene y añade una imagen más al ciclo de publicación. Hay que decidir **quién lo
  mantiene** antes de empezar, no después.
- **Alternativa más barata: un TUI.** Es lo que DDEV eligió, y es lo que sigue vivo. Un
  `hm` sin argumentos que muestre la flota con atajos cubre buena parte del valor de la
  Fase 1 sin servidor, sin imagen y sin superficie de seguridad. **Merece evaluarse
  seriamente frente a la web**, o incluso hacerse antes.
- **PHP+htmx frente a binario Go.** Sin decidir (§5.4).
- **Riesgo de duplicar Docker Desktop.** Se conjura con la regla del §4: si una pantalla no
  habla de proyectos, worktrees, agentes o snapshots, no entra.
- **Deriva CLI ↔ web.** Se conjura con la regla del §3: la web no implementa lógica; sólo
  consume el contrato y llama a los mismos verbos.

## 10. Recomendación

1. Hacer la **Fase 0** ya: es común a las tres líneas de investigación y aporta valor sin
   comprometer nada.
2. **Prototipar el TUI** (`hm` sin argumentos) antes que la web. Es de horas, no de
   semanas, y responde a la pregunta de cuánta de la necesidad es "ver la flota" y cuánta
   es "operarla cómodamente".
3. Abordar la **Fase 1 web sólo lectura** cuando exista el proxy global, que es lo que
   crea de verdad el escenario multi-proyecto.
4. Decidir explícitamente el mantenedor y el stack antes de la Fase 2.
