## Why

No hay forma de preguntarle a Dockergento **qué es este proyecto** ni **qué hay levantado en
esta máquina**. Para saber la URL, el puerto de la base de datos, la versión de Magento o
qué proyectos están arriba hay que leer `config/docker/properties.json`, cruzarlo con
`data/requirements.json` y rematar con `docker ps`. Eso lo sufre a diario quien da soporte
interno, y lo sufre todavía más un agente de IA, que acaba adivinando —o inventándose— las
URLs y los nombres de contenedor.

Estos dos comandos son **el contrato del entorno**: la misma salida la van a consumir la
persona en el terminal, el TUI, el futuro dashboard y los agentes vía MCP. Es la pieza que
se paga una vez y se cobra tres.

Backlog: **CLI-02** y **CLI-03**.

## What Changes

- Nuevo comando `hm describe [--json] [--with-secrets]`: todo lo que define el proyecto del
  directorio actual — dominio y URLs, nombre del proyecto Compose, versión de Magento,
  versión de cada servicio, nombres y estado de los contenedores, rutas relevantes, modo de
  despliegue, estado de Xdebug y, sólo bajo demanda, las credenciales.
- Nuevo comando `hm list [--json]`: inventario de todos los entornos Dockergento de la
  máquina, levantados o no, con su estado, su ruta de origen y si son checkout principal o
  worktree.
- Ambos respetan el contrato de salida de `add-cli-output-contract` y usan el
  descubrimiento por etiquetas de `add-compose-project-labels`.
- Se documenta el esquema JSON de la respuesta, versionado con `schema_version`.

## Non-goals

- No se implementa el TUI ni ninguna interfaz: sólo los dos comandos.
- No se añaden acciones: son comandos estrictamente de lectura.
- No se implementa `hm doctor`, que va en su propio cambio, aunque `describe` señale
  cuando algo no está levantado.
- No se toca el formato de `config/docker/properties.json`.

## Capabilities

### New Capabilities
- `environment-introspection`: qué información expone Dockergento sobre un proyecto y sobre
  la flota de entornos de la máquina, y con qué forma la expone.

### Modified Capabilities
<!-- Ninguna: consume `cli-output-contract` y `environment-discovery` sin cambiar sus requisitos. -->

## Impact

- **Código**: nuevos `console/commands/describe.sh` y `console/commands/list.sh`; nueva
  tarea compartida en `console/tasks/` para reunir los datos del proyecto;
  `console/helpers/docker.sh` (descubrimiento).
- **Datos**: `data/command_descriptions.json`.
- **Documentación**: `docs/describe.md`, `docs/list.md` y el esquema JSON.
- **Proyectos existentes**: no requiere migración. `hm list` mostrará como "sin metadatos"
  los entornos que aún no tengan las etiquetas `hm.*`.
- **Dependencias**: ninguna nueva. Depende de los cambios `add-cli-output-contract` y
  `add-compose-project-labels`.
