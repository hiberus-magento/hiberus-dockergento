# environment-cleanup Specification

## Purpose
TBD - created by archiving change add-clean-command. Update Purpose after archive.
## Requirements
### Requirement: Mirar no borra

`hm clean` SHALL informar de lo que se puede recoger sin eliminar nada, salvo que se pida
explícitamente lo contrario.

#### Scenario: Sin opciones
- **WHEN** se ejecuta el comando sin opciones
- **THEN** informa de lo recogible y no elimina ningún contenedor ni ningún volumen

#### Scenario: Nada que recoger
- **WHEN** no hay entornos abandonados
- **THEN** lo dice y termina correctamente

#### Scenario: Para una máquina
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** la respuesta es el sobre JSON habitual, con lo recogible y lo no atribuible por separado

### Requirement: Sólo se recoge lo demostrablemente abandonado

`hm clean` SHALL considerar recogible únicamente un entorno creado por esta herramienta cuyo
directorio de origen ya no exista.

#### Scenario: Un proyecto cuyo directorio existe
- **WHEN** el entorno está parado pero su directorio sigue en el disco
- **THEN** no aparece entre lo recogible, en ningún caso

#### Scenario: Un proyecto en marcha
- **WHEN** el entorno tiene contenedores funcionando
- **THEN** no aparece entre lo recogible

#### Scenario: Un proyecto cuyo directorio desapareció
- **WHEN** el entorno tiene etiquetas de esta herramienta y su directorio de origen ya no existe
- **THEN** aparece entre lo recogible, con sus contenedores y sus volúmenes

#### Scenario: Un entorno sin etiquetas de esta herramienta
- **WHEN** el entorno no lleva etiquetas que permitan atribuirlo
- **THEN** no se recoge, y se informa aparte de que no se puede atribuir

### Requirement: Los volúmenes se atribuyen por sus contenedores

`hm clean` SHALL eliminar únicamente volúmenes cuyo proyecto pueda demostrarse que es de esta
herramienta.

#### Scenario: Volúmenes de un proyecto atribuible
- **WHEN** un proyecto abandonado conserva contenedores con etiquetas de esta herramienta
- **THEN** sus volúmenes se consideran recogibles

#### Scenario: Volúmenes sin contenedores que los expliquen
- **WHEN** existen volúmenes de un proyecto del que no queda ningún contenedor
- **THEN** se listan aparte como no atribuibles y no se eliminan, ni siquiera al pedir que se borre

### Requirement: Borrar exige pedirlo y confirmarlo

`hm clean` SHALL eliminar recursos únicamente cuando se pida de forma explícita y se confirme.

#### Scenario: Se enumera antes de borrar
- **WHEN** se pide borrar
- **THEN** se enumera lo que se va a eliminar y el espacio que se va a liberar antes de preguntar

#### Scenario: No confirmar no borra
- **WHEN** no se confirma
- **THEN** no se elimina nada

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se elimina lo recogible sin preguntar

### Requirement: Lo ajeno se informa, no se toca

`hm clean` SHALL informar de los recursos de Docker que no puede atribuir a esta herramienta sin
eliminarlos ni delegar su eliminación.

#### Scenario: Recursos de Docker que no son de esta herramienta
- **WHEN** hay imágenes sueltas o caché de construcción ocupando espacio
- **THEN** se informa de cuánto ocupan y de con qué comando se limpian, sin ejecutarlo

#### Scenario: Las copias de base de datos no entran
- **WHEN** se recoge un entorno abandonado
- **THEN** sus copias de base de datos no se eliminan

