# Research

Línea de investigación abierta sobre la evolución de Hiberus Dockergento. Documentos de
trabajo: salvo donde la tabla diga lo contrario, **ninguno describe funcionalidad
implementada**.

| Documento | Tema | Estado |
|---|---|---|
| **[backlog.md](backlog.md)** | **Lista consolidada de ítems candidatos, con IDs, esfuerzo, dependencias y criterios de aceptación** | **Vivo — es el documento del que se elige** |
| [git-worktrees.md](git-worktrees.md) | Trabajo en paralelo con git worktrees y varios agentes: diagnóstico con evidencias, estado del arte y estrategia por niveles de entorno | Propuesta, con PoC pendiente |
| [ddev-warden-feature-mining.md](ddev-warden-feature-mining.md) | Funcionalidades de DDEV y Warden que merece la pena incorporar a Dockergento | Catálogo priorizado |
| [gentle-ai-engram-mining.md](gentle-ai-engram-mining.md) | Patrones de herramientas CLI en Go (gentle-ai, engram) aplicables a la migración a Go: distribución, instalador, release, y qué **no** copiar de su arquitectura | Catálogo priorizado — para la 2.0 |
| [ai-features.md](ai-features.md) | Qué incorporar a Dockergento por el lado de la IA: modo agente, MCP, verificación, datos seguros | Catálogo priorizado |
| [terminal-ux.md](terminal-ux.md) | Interfaz de terminal y experiencia de uso: colores, preguntas, ayuda, progreso y la base del TUI | Propuesta |
| [tui-landscape.md](tui-landscape.md) | Cómo se hace un TUI en Bash: qué han resuelto bashtop y fff, qué alternativas hay (fzf, gum, dialog), y las seis reglas de render que salieron de medir el nuestro | **Implementado** — las reglas están en `console/tasks/tui_frame.sh` |
| [control-plane-ui.md](control-plane-ui.md) | Dashboard y plano de control local servido por el proxy global: qué es, qué no es, arquitectura y fases | Propuesta de arquitectura |

## Decisiones tomadas

Seguimos con Dockergento (no migramos a Warden ni DDEV) · **TUI antes que web** · `share`
con **Cloudflared**, no ngrok · `hm` no llama a modelos de IA · sin registro de add-ons ·
sin Portainer. El detalle y el motivo de cada una, en [backlog.md](backlog.md).

## Hilo conductor

Las tres investigaciones convergen en las mismas cuatro piezas:

1. **`hm doctor` / `hm describe --json`** — introspección del entorno; base de la DX y de
   todo lo relacionado con agentes.
2. **`hm db snapshot` / `restore`** — red de seguridad para migraciones y requisito para
   clonar entornos por worktree.
3. **Proxy global compartido** — desbloquea multi-proyecto simultáneo (hoy imposible) y es
   prerrequisito de los entornos por agente.
4. **`hm verify`** — la puerta de calidad que convierte el trabajo de los agentes en algo
   revisable.

Y una quinta que sale de las cuatro anteriores: el **contrato JSON** (`describe`/`list`)
tiene tres consumidores —la CLI, los agentes vía MCP y el dashboard— así que se paga una
vez y se cobra tres.
