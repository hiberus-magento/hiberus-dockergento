# Etiquetas `hm.*` (tareas 1.1 y 1.2)

Se estampan en **todos** los servicios del entorno. Sólo contienen identidad estable: lo
volátil (rama, estado de git, última actividad) se deriva en tiempo de lectura a partir
de `hm.root`.

| Etiqueta | Contenido | Origen |
|---|---|---|
| `hm.project` | Nombre del proyecto Compose | `COMPOSE_PROJECT_NAME` |
| `hm.root` | Ruta absoluta en el host del checkout desde el que se levantó | `$PWD` (con WT-01 pasará a ser el checkout principal) |
| `hm.worktree` | Identificador del worktree; vacío en el checkout principal | vacío hasta WT-01 |
| `hm.profile` | Perfil del entorno | `full` por defecto |
| `hm.magento` | Versión de Magento instalada | `composer.lock` |
| `hm.version` | Versión de `hm` que creó el entorno | `git describe` sobre la instalación |
| `hm.agent` | Agente que ocupa el entorno, opcional | variable de entorno `HM_AGENT` |

## Decisión 1.2 · valor por defecto de `hm.profile`

`full`. Los perfiles (`lite`, `agent`, `full`) no llegan hasta WT-02, pero la etiqueta se
introduce ya con valor por defecto para no tener que volver a tocar la plantilla ni
recrear contenedores más adelante.

## Coste medido (tarea 3.3)

Leer la versión de Magento de un `composer.lock` de 1,6 MB con `jq`: **55 ms**. Frente al
`docker compose config` que ya ejecuta la validación de proyecto en cada invocación, es
ruido. No se cachea.
