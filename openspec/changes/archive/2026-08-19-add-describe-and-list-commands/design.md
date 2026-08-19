## Context

La información que necesitan estos comandos está hoy repartida en cuatro sitios:
`config/docker/properties.json` del proyecto (nombre, dominio, directorio de Magento),
`data/requirements.json` (versiones de servicio por versión de Magento),
`composer.lock` del proyecto (versión real de Magento instalada) y el propio Docker
(estado de los contenedores, puertos publicados).

`hm describe` opera sobre el proyecto del directorio actual, igual que el resto de la CLI.
`hm list` es distinto: debe funcionar **desde cualquier sitio**, incluso fuera de un
proyecto, porque su fuente es la máquina y no el directorio.

## Goals / Non-Goals

**Goals:**
- Una única fuente de verdad sobre "qué es este entorno", consumible por personas y por
  máquinas.
- Que sea útil también con el entorno parado.
- Que las credenciales no se filtren por defecto.

**Non-Goals:**
- Diagnosticar problemas: eso es `hm doctor`.
- Modificar nada.

## Decisions

### 1. `describe` funciona con el entorno parado

La mayor parte de la información (dominio, versiones, rutas, nombre de proyecto) sale de
ficheros, no de Docker. Sólo el estado y los puertos publicados requieren contenedores
vivos. Con el entorno parado se devuelve todo lo demás y el estado indica `stopped`.

Alternativa descartada: exigir el entorno levantado. Rompería el caso de uso más frecuente
de soporte, que es justamente "no me arranca, ¿qué tengo aquí?".

### 2. Las credenciales no salen por defecto

`describe` omite contraseñas salvo `--with-secrets`. En su lugar informa de **dónde**
están. Motivo: la salida de este comando va a acabar dentro del contexto de agentes de IA y
en tickets de soporte; hoy son credenciales fijas y conocidas de desarrollo, pero el hábito
correcto se establece desde el principio, no cuando duela.

### 3. Esquema versionado y plano

`schema_version` desde el primer día (heredado del contrato de salida). El esquema se
mantiene plano y explícito, con claves estables, aunque eso implique repetir información:
lo van a consumir un TUI en Bash, una web y un servidor MCP, y ninguno debe tener que
recomponer datos.

Bloques de `describe`: `project` (nombre, root, worktree, dominio, urls), `magento`
(versión, edición, modo de despliegue), `services` (por servicio: imagen, versión, estado,
puertos), `paths`, `tooling` (xdebug, versión de `hm`) y `credentials` (sólo con
`--with-secrets`).

### 4. `list` no depende del directorio actual

Se apoya exclusivamente en el descubrimiento por etiquetas. Como consecuencia, un proyecto
que nunca se ha levantado **no aparece**: no hay contenedores que etiquetar. Se asume y se
documenta; la alternativa —mantener un registro de proyectos conocidos en `~/.hm/`— añade
estado que se desincroniza, que es justo lo que se evitó en `add-compose-project-labels`.

### 5. Versión de Magento: fichero primero, contenedor después

Se lee de `composer.lock`, que es lo que ya hace `version_manager.sh`, y no se ejecuta
`bin/magento --version` dentro del contenedor: es lento y exige el entorno arriba. Si no
hay `composer.lock`, se informa como desconocida en vez de fallar.

### 6. Salida de texto pensada para leerse, no para parsearse

En modo texto se agrupa por bloques, con las URLs y el estado arriba, que es lo que se
consulta el 90 % de las veces. Quien quiera parsear tiene `--json`; nadie debería estar
haciendo `grep` sobre la salida de texto, y documentarlo así evita crear un contrato
implícito.

### 7. mac y linux

Sin diferencias de comportamiento. La única variación es informativa: `paths` refleja la
estrategia de montaje de cada plataforma (volumen `workspace` más binds en mac, bind
directo en linux), porque es información que se pide constantemente en soporte.

## Risks / Trade-offs

- **El esquema se convierte en contrato público en cuanto lo consuma alguien** → por eso
  `schema_version` desde el principio y documentación explícita de qué claves son estables.
- **Coste de arranque**: reunir los datos implica varias llamadas a `docker` y a `jq`. Si se
  nota en el TUI, que refresca a menudo, habrá que cachear; se mide antes de optimizar.
- **`list` no ve proyectos nunca levantados** → documentado; se puede revisar cuando exista
  el proxy global, que sí tendrá una vista de la máquina.
- **Fuga de credenciales por costumbre** → el flag explícito y la ausencia por defecto
  reducen el riesgo, pero no lo eliminan si alguien pega la salida con `--with-secrets`.

## Migration Plan

No aplica: son comandos nuevos, de sólo lectura. Si algo falla, no ejecutarlos.

## Open Questions

- ¿`hm describe` debe aceptar el nombre de otro proyecto como argumento
  (`hm describe otro-proyecto`) o limitarse al directorio actual? Propuesta: en esta
  iteración, sólo el directorio actual; `hm list` cubre la vista global.
- ¿Merece la pena un `--format=table|json|env` con salida en variables de shell? Se pospone
  hasta que haya un consumidor real que lo pida.
