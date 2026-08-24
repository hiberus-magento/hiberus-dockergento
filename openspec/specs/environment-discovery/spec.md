# environment-discovery Specification

## Purpose
TBD - created by archiving change add-compose-project-labels. Update Purpose after archive.
## Requirements
### Requirement: Metadatos en los contenedores del entorno

Todos los servicios de un entorno Dockergento SHALL llevar etiquetas con el prefijo `hm.`
que identifiquen el entorno al que pertenecen.

#### Scenario: Entorno recién creado
- **WHEN** se levanta un entorno generado con esta versión de la plantilla
- **THEN** todos sus contenedores llevan las etiquetas `hm.project`, `hm.root`, `hm.worktree`, `hm.profile`, `hm.magento` y `hm.version`

#### Scenario: Servicio parado
- **WHEN** uno de los servicios del entorno está parado y el resto levantado
- **THEN** el entorno sigue siendo descubrible a partir de las etiquetas de los servicios restantes

#### Scenario: Valores no definidos
- **WHEN** alguna de las variables que alimenta una etiqueta no está definida en el entorno de ejecución
- **THEN** `docker compose config` sigue siendo válido y la etiqueta queda vacía

### Requirement: Identidad estable frente a datos volátiles

Las etiquetas SHALL contener únicamente información que no cambie durante la vida del
contenedor. La información volátil SHALL derivarse en tiempo de lectura a partir de
`hm.root`.

#### Scenario: Cambio de rama
- **WHEN** se cambia de rama en el checkout desde el que se levantó el entorno
- **THEN** las etiquetas del contenedor no cambian y siguen siendo correctas

#### Scenario: Consulta de la rama actual
- **WHEN** una herramienta necesita saber en qué rama está un entorno
- **THEN** la obtiene consultando git en la ruta indicada por `hm.root`

#### Scenario: Distinción entre checkout principal y worktree
- **WHEN** el entorno se levantó desde el checkout principal
- **THEN** `hm.worktree` está vacía
- **WHEN** el entorno se levantó desde un worktree
- **THEN** `hm.worktree` contiene el identificador de ese worktree

### Requirement: Descubrimiento de entornos por etiquetas

La CLI SHALL ofrecer una función de descubrimiento que enumere los entornos Dockergento de
la máquina a partir de las etiquetas, sin depender de coincidencias por nombre de
contenedor.

#### Scenario: Varios proyectos en la máquina
- **WHEN** hay varios entornos Dockergento levantados y se pide el inventario
- **THEN** se obtiene un entorno por cada `hm.project` distinto, con sus servicios agrupados

#### Scenario: Contenedores ajenos
- **WHEN** en la máquina hay contenedores que no pertenecen a Dockergento
- **THEN** no aparecen en el inventario

#### Scenario: Resolución de un servicio concreto
- **WHEN** se pide el contenedor de un servicio dentro de un proyecto concreto
- **THEN** se devuelve únicamente el contenedor de ese proyecto, aunque otros proyectos tengan un servicio con el mismo nombre

### Requirement: Compatibilidad con entornos anteriores

El descubrimiento SHALL seguir encontrando entornos creados antes de la introducción de las
etiquetas.

#### Scenario: Entorno sin etiquetas hm
- **WHEN** hay un entorno levantado cuyos contenedores no tienen etiquetas `hm.*`
- **THEN** se descubre por las etiquetas estándar de Compose
- **AND** se marca como entorno sin metadatos

#### Scenario: Incorporación de metadatos
- **WHEN** se regenera la configuración de ese proyecto y se recrean sus contenedores
- **THEN** el entorno pasa a tener todas las etiquetas `hm.*`

### Requirement: Detección de entornos huérfanos

El descubrimiento SHALL permitir identificar entornos cuyo directorio de origen ya no
existe.

#### Scenario: Directorio de origen borrado
- **WHEN** un entorno tiene una ruta en `hm.root` que ya no existe en el sistema de ficheros
- **THEN** el descubrimiento lo señala como huérfano
- **AND** no lo elimina ni lo detiene por su cuenta

### Requirement: Nombre de proyecto resuelto

La CLI SHALL operar siempre con un nombre de proyecto que coincida con el que usa Docker Compose.

#### Scenario: El nombre configurado manda
- **WHEN** el proyecto tiene `COMPOSE_PROJECT_NAME` con valor en sus propiedades
- **THEN** ese es el nombre del entorno, y nada lo deriva ni lo modifica

#### Scenario: Sin nombre configurado, lo pone el directorio
- **WHEN** la propiedad está vacía o no existe
- **THEN** el nombre se deriva del directorio raíz del proyecto siguiendo la misma regla que
  aplica Docker Compose

#### Scenario: La derivación coincide con la de Compose
- **WHEN** el directorio contiene mayúsculas, espacios, puntos o caracteres acentuados
- **THEN** el nombre derivado es idéntico al que `docker compose config` informa para ese mismo
  directorio

#### Scenario: Un worktree no inventa una identidad nueva
- **WHEN** el comando se ejecuta desde un worktree de un proyecto sin nombre configurado
- **THEN** el nombre derivado es el del checkout principal, que es el entorno sobre el que se
  está operando

#### Scenario: Un directorio sin nombre válido
- **WHEN** del directorio no sale ningún carácter admisible
- **THEN** el comando falla con el código de proyecto y explica las dos salidas: renombrar el
  directorio o fijar el nombre en las propiedades

#### Scenario: Las etiquetas llevan el nombre resuelto
- **WHEN** se crean los contenedores de un proyecto sin nombre configurado
- **THEN** la etiqueta de proyecto lleva el nombre derivado, no una cadena vacía

#### Scenario: Los entornos se descubren por él
- **WHEN** se listan los entornos de la máquina o se busca el contenedor de un servicio
- **THEN** la búsqueda usa el nombre resuelto, de modo que un proyecto sin nombre configurado se
  encuentra igual que uno que lo tenga

### Requirement: Identidad separada por directorio

Dos copias del mismo repositorio en directorios distintos SHALL ser dos entornos independientes
mientras ninguna fije un nombre.

#### Scenario: Dos clones
- **WHEN** se clona un proyecto sin nombre configurado en otro directorio y se levanta
- **THEN** sus contenedores y sus volúmenes son distintos de los del original

#### Scenario: Un nombre fijado se hereda al clonar
- **WHEN** el proyecto tiene el nombre fijado en sus propiedades versionadas
- **THEN** el clon comparte identidad con el original, porque el nombre es una decisión explícita
  del equipo

### Requirement: `setup` guarda el nombre sólo cuando es una decisión

`hm setup` SHALL escribir `COMPOSE_PROJECT_NAME` en las propiedades del proyecto únicamente
cuando el nombre elegido difiera del que se derivaría del directorio.

#### Scenario: Se acepta el nombre propuesto
- **WHEN** se acepta el valor por defecto, que es el derivado del directorio
- **THEN** la propiedad no se escribe, y el proyecto queda sin nombre fijado

#### Scenario: Se escribe un nombre propio
- **WHEN** se introduce un nombre distinto del derivado
- **THEN** la propiedad se escribe con ese nombre

#### Scenario: Un proyecto que ya tenía nombre
- **WHEN** se vuelve a ejecutar `setup` sobre un proyecto cuyas propiedades ya fijan un nombre
- **THEN** ese nombre se respeta y sigue escrito

