# Línea base medida (tareas 1.1 – 1.3)

macOS 24.6.0, Apple Silicon, Docker Desktop 28.0.2, proyecto `example-shop` con los
9 servicios levantados. Mejor de tres intentos.

| Ruta | Antes | Después | Procesos antes → después |
|---|---|---|---|
| `hm --help` | 5 660 ms | **390 ms** | 143 `jq` → **3 `jq`** |
| `hm doctor --json` | 3 850 ms | **1 300 ms** | 12 checks en serie → en paralelo |
| `hm list --json` | 1 160 ms | 890 ms | — |
| `hm describe --json` | 1 220 ms | 1 160 ms | — |
| `hm --version` (suelo) | 300 ms | 280 ms | 4 `git` → 2 `git` |

## Coste unitario

| Operación | Coste |
|---|---|
| Arrancar `bash` | 76 ms |
| Cargar los ficheros de código con `source` | ~100-160 ms |
| Un proceso `jq` | ~36 ms |
| `jq` sobre `composer.lock` (1,6 MB) | 77 ms |
| `git rev-parse` × 3 (como está hoy) | 132 ms |
| `git rev-parse` × 1 (combinada) | 46 ms |
| `git describe --tags` | 38-56 ms |
| `docker compose version` | 188 ms |
| `docker info` | 78 ms |
| `docker compose config -q` | 72 ms en caliente, **325 ms en frío** |

## Conclusión

El coste de la CLI es el número de procesos que lanza. El listado de comandos consulta
143 veces el mismo fichero de 13 KB.

## Después: lo que se quedó fuera

- **El coste de `source`** (~100-160 ms del suelo). Reducirlo obliga a cargar ficheros de
  forma perezosa y cambia cuándo está definida cada función. No merece el riesgo.
- **La fusión de las llamadas a git** se hizo, pero midiendo A/B da **~14 ms**, no los 86 ms
  estimados en aislamiento. Se mantiene por simplicidad de código, no por tiempo.
- **`hm describe` apenas mejora** porque su coste es real: consulta a Docker el estado de
  los contenedores y entra en el contenedor de PHP para saber si Xdebug está activo.
