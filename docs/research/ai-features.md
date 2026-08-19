# Investigación: Dockergento en la era de los agentes

> Estado: **investigación / catálogo de features candidatas**. Nada implementado.
> Relacionado: [git-worktrees.md](git-worktrees.md) (entornos por agente) y
> [ddev-warden-feature-mining.md](ddev-warden-feature-mining.md) (features de terceros).

---

## 1. De dónde partimos

Dockergento **ya tiene una capa de IA**: `hm ai-init`, `hm ai-pull` y `hm ai-reset`
descargan skills y agentes desde repositorios (`hiberus-magento/ai-tools`,
`hyva-themes/hyva-ai-tools`) y los instalan en el directorio de cada plataforma
(`.claude/`, `.cursor/`, `.codex/`, `.github/`, `.gemini/`, `.opencode/`), con tipos por
dominio (magento, php, hyva, acs, dockergento).

Eso cubre **la capa de prompt**: qué sabe el agente. Lo que sigue abierto son las otras
dos capas:

| Capa | Pregunta | Estado |
|---|---|---|
| **Prompt** | ¿Qué sabe el agente sobre Magento y sobre nuestro modo de trabajar? | ✅ `hm ai-*` |
| **Herramienta** | ¿Cómo *opera* el agente el entorno de forma fiable y segura? | ❌ vacío |
| **Entorno** | ¿Dónde ejecuta cada agente sin pisar a los demás? | ❌ vacío ([worktrees](git-worktrees.md)) |

Este documento va de la **capa de herramienta**. Y hay un motivo para priorizarla: es
justo donde la comunidad Magento no está mirando — todo lo publicado (colecciones de
subagentes, skills, servidores MCP de tienda) vive en la capa de prompt.

## 2. Principio rector: Dockergento ya es la herramienta del agente

Un agente que trabaja en un proyecto Magento **ya usa `hm`** — es la única vía para
ejecutar `bin/magento`, `composer` o consultar la BD. La pregunta no es si añadimos IA a
Dockergento, sino si Dockergento se comporta bien cuando quien lo llama no es una persona.

Hoy no siempre se comporta bien. Caso real, corregido en la 1.4.5: `hm mysql` trataba
*cualquier* stdin sin tty como un volcado a importar, así que un agente que ejecutaba
`hm mysql -q "..."` nunca llegaba al parser de opciones. Ese bug es el síntoma de un
patrón: **la CLI asume terminal interactiva**.

Los principios de diseño de CLIs para agentes que se han consolidado en 2026 son
exactamente el remedio:

- salida legible por humanos **y** estructurada (`--json`), conmutando por defecto según
  si stdout es un TTY;
- funcionamiento **no interactivo** sin preguntas (`--yes` / `--no-interactive`);
- **códigos de salida** significativos y errores estructurados en stderr;
- y, cuando aplica, **un servidor MCP o una skill** que acompañe a la CLI.

Motivo económico, no sólo estético: la salida estructurada evita que el modelo gaste
tokens interpretando tablas, colores y prosa, con menos latencia y menos reintentos.

## 3. Features candidatas

### 3.1 Modo agente en toda la CLI — *la base de todo*

- `--json` en los comandos de lectura (`describe`, `list`, `doctor`, `compatibility`,
  `db list`), con salida por defecto en JSON cuando stdout no es un TTY.
- `--yes` / `HM_NON_INTERACTIVE=1` para que `setup`, `install` y los asistentes no se
  queden esperando una respuesta que nadie va a escribir. Hoy `input_info.sh` pregunta
  siempre que falte un dato, y existe `USE_DEFAULT_SETTINGS` pero sólo en parte del flujo.
- Auditoría de todos los comandos buscando el patrón `[ -t 0 ]` y los `read -rp`.
- Códigos de salida distintos para "docker parado", "proyecto no configurado" y
  "servicio caído", en lugar del `exit 1` genérico.

**Esfuerzo bajo, valor alto**: sin esto, el resto de la línea de IA es inestable.

### 3.2 `hm describe --json` como *contrato del entorno*

Un único comando que devuelva: URL del proyecto, dominio, nombre del proyecto compose,
versión de Magento y de cada servicio, nombres de contenedor, credenciales de BD, estado
de Xdebug, rutas relevantes y modo de despliegue.

Con eso encima se construye casi todo lo demás:

- **Generar/refrescar el bloque de entorno en `CLAUDE.md` / `AGENTS.md`** del proyecto, en
  lugar de que cada agente lo descubra a base de comandos (y a menudo se lo invente).
- Alimentar la configuración MCP (§3.3).
- Que un agente sepa **a qué URL pedir** para verificar un cambio.

### 3.3 `hm mcp`: servidor MCP del entorno

Exponer Dockergento como herramientas MCP en lugar de obligar al agente a componer
comandos de shell. Precedentes en DDEV: add-ons de asistentes (Claude Code, Copilot,
Cursor) y `ddev-codebase-memory-mcp`, que sirve un grafo del código como servidor MCP.

Herramientas candidatas, separadas por nivel de riesgo:

| Nivel | Herramientas |
|---|---|
| **Lectura** | `describe`, `list`, `logs`, `db.query` (sólo `SELECT`), `magento.config_show`, `indexer.status`, `cache.status` |
| **Escritura acotada** | `cache.clean`, `cache.flush`, `reindex`, `magento.config_set`, `composer.show` |
| **Peligrosas — requieren confirmación humana** | `setup:upgrade`, `composer require/update`, `db.import`, `down -v`, `install` |

El valor no es "poder ejecutar": ya puede. El valor es **el esquema tipado, la salida
estructurada y la clasificación por riesgo**, que es lo que hoy no existe en ningún sitio.

### 3.4 `hm verify`: la puerta de calidad que el agente debe pasar

La guía de operación de flotas de agentes es tajante: *el cuello de botella ya no es
generar, es verificar*, y lo que funciona son **puertas de calidad ejecutadas por la
máquina**, no por el criterio del modelo.

Propuesta: `hm verify [--changed] [--json]` que ejecute dentro del contenedor la batería
que ya usamos suelta — PHPCS con el estándar Magento2, PHPStan/Rector, `test-unit`,
compilación de DI, validación de XML/XSD — y devuelva un resultado estructurado.

Encaja de dos formas: como comando manual, y como **hook** de Claude Code al cerrar tarea
(salida distinta de cero bloquea). Es, con diferencia, la feature con mejor relación entre
esfuerzo y calidad del código que producen los agentes.

### 3.5 Verificación visual: servicio de navegador

Para Magento y sobre todo para Hyvä, buena parte del trabajo es frontend, y un agente sin
navegador no puede comprobar lo que ha hecho. Un servicio opcional de Playwright dentro de
la red del proyecto —como el add-on de DDEV— permite `hm screenshot <ruta>` o un
`hm e2e`, resolviendo por DNS interno (`nginx`) en lugar de por `localhost`, que es
justamente lo que rompe en contenedores.

Comparte infraestructura con la necesidad ya existente de MFTF/Selenium
([feature mining](ddev-warden-feature-mining.md) §3.8).

### 3.6 Datos seguros para agentes — apoyándonos en `masquerade`

Dockergento **ya trae anonimización** (`hm masquerade`, `hm mysql -a`). Con agentes esto
deja de ser una buena práctica y pasa a ser una cuestión de cumplimiento: los volcados de
clientes llevan datos personales y un agente los mete en su contexto sin pensarlo.

Propuestas:

- `hm mysql -i --anonymise-by-default` para entornos marcados como "de agente".
- Generar un fichero de exclusión de contexto (`.claudeignore` / equivalente) con lo que
  nunca debe entrar: `app/etc/env.php`, `var/log/*`, `pub/media/customer/*`, `vendor/`,
  `generated/`, `var/cache/`. Es exactamente lo que hacen las agencias a mano.
- Que `hm describe --json` **no** vuelque credenciales salvo con `--with-secrets`.

### 3.7 Permisos y guardarraíles por comando

Clasificar cada comando de `hm` como *seguro sin supervisión* (`cache:clean`, `exec`,
`mysql -q`, `logs`) frente a *requiere confirmación* (`setup:upgrade`, `composer update`,
`down -v`, `install`, `docker-stop-all`), y **generar desde ahí la configuración de
permisos** de la plataforma (por ejemplo `.claude/settings.json`). Hoy cada persona del
equipo mantiene esa lista a mano, cada una distinta.

Se apoya en el mismo trabajo que los guardarraíles de worktree
([git-worktrees.md](git-worktrees.md) §6).

### 3.8 Contexto del código para Magento

Magento es enorme y los agentes queman contexto navegándolo. Un índice del proyecto
—módulos, plugins, preferencias, observers, layouts, dónde está declarado cada `di.xml`—
servido como MCP evitaría media docena de `grep` por consulta. Existe precedente directo
(`ddev-codebase-memory-mcp`), y el dominio Magento es especialmente propicio porque la
estructura es muy regular.

Es la propuesta más ambiciosa de la lista y la que más conviene validar con un prototipo
antes de comprometerse.

### 3.9 Evolución de la capa `ai-*` que ya existe

- `hm ai-doctor`: qué skills hay instaladas, de qué repositorio, qué versión y si están al
  día — hoy `ai-pull --force` es a ciegas.
- **Fijar versiones** de los repositorios de skills (hoy se sigue una rama).
- `hm ai-init` debería poder **generar también la configuración MCP** (`.mcp.json`) y el
  `CLAUDE.md`/`AGENTS.md` base del proyecto con los datos de `describe`.
- Un tipo de skill nuevo, `dockergento`, ya existe: es el sitio natural para documentar
  a los agentes **cómo se opera este entorno** (que no lo adivinen).

## 4. Priorización propuesta

| Prioridad | Feature | Esfuerzo | Por qué primero |
|---|---|---|---|
| 1 | Modo agente: `--json`, `--yes`, exit codes | Bajo | Todo lo demás depende de esto |
| 2 | `hm describe --json` | Bajo | Contrato del entorno; alimenta MCP y CLAUDE.md |
| 3 | `hm verify` | Medio | Ataca el cuello de botella real: la verificación |
| 4 | Permisos y guardarraíles generados | Bajo | Evita destrozos y unifica criterio del equipo |
| 5 | `hm mcp` (sólo lectura primero) | Medio | Operación fiable y barata en tokens |
| 6 | Datos seguros / anonimización por defecto | Bajo | Cumplimiento, y ya tenemos `masquerade` |
| 7 | Servicio de navegador (Playwright) | Medio | Verificación de frontend, MFTF |
| 8 | `hm ai-doctor` y versionado de skills | Bajo | Mantenimiento de lo ya construido |
| 9 | Índice de código como MCP | Alto | Prototipo antes de decidir |

## 5. Criterio para descartar

No todo lo que lleve "IA" encaja en una herramienta de entorno:

- **Nada de llamar a modelos desde `hm`.** Dockergento debe ser *operado por* agentes, no
  *contener* un agente. Meter claves de API y llamadas a LLM en la CLI añade coste,
  dependencia de proveedor y superficie de seguridad, y duplica lo que ya hacen Claude
  Code, Cursor o Codex.
- **Nada de features de "IA en la tienda"** (recomendadores, chatbots, agentic commerce).
  Es otro producto: aquí hablamos del entorno de desarrollo.
- **Nada de observabilidad de coste de tokens.** Lo resuelven las plataformas.

La regla: *Dockergento aporta el entorno, la verificación y la verdad sobre el proyecto.
El modelo lo pone otro.*
