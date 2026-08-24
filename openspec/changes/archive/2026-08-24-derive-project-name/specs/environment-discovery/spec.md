## ADDED Requirements

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
