## ADDED Requirements

### Requirement: Guardar la base de datos con nombre

`hm db snapshot` SHALL guardar una copia de la base de datos del proyecto sin interrumpirlo.

#### Scenario: Copia con nombre
- **WHEN** se pide una copia indicando un nombre
- **THEN** queda guardada bajo ese nombre y asociada a este proyecto

#### Scenario: Copia sin nombre
- **WHEN** no se indica ninguno
- **THEN** se usa la fecha y la hora, de forma que dos copias seguidas no se pisan

#### Scenario: El proyecto sigue en marcha
- **WHEN** se hace una copia con el entorno levantado
- **THEN** los servicios siguen funcionando y la base de datos no se modifica

#### Scenario: Un nombre que ya existe
- **WHEN** se pide una copia con el nombre de una que ya existe
- **THEN** el comando se niega en lugar de sobrescribirla, salvo que se le indique lo contrario

#### Scenario: Un nombre inservible
- **WHEN** el nombre contiene caracteres que no pueden formar parte de un fichero
- **THEN** el comando falla como error de uso y explica qué se admite

### Requirement: Ver las copias que hay

`hm db list` SHALL informar de las copias del proyecto actual.

#### Scenario: Qué informa
- **WHEN** se listan las copias
- **THEN** de cada una se indica su nombre, cuándo se hizo y cuánto ocupa

#### Scenario: Sólo las de este proyecto
- **WHEN** existen copias de varios proyectos
- **THEN** sólo aparecen las del proyecto actual

#### Scenario: Sin copias
- **WHEN** el proyecto no tiene ninguna
- **THEN** se dice, y se indica cómo crear la primera

#### Scenario: Para una máquina
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** la respuesta es el sobre JSON habitual con los mismos datos

### Requirement: Volver a una copia

`hm db restore` SHALL dejar la base de datos como estaba cuando se hizo la copia.

#### Scenario: Restauración exacta
- **WHEN** se restaura una copia sobre una base de datos que ha cambiado desde entonces
- **THEN** el resultado es exactamente el contenido de la copia, sin restos de lo que hubiera
  después

#### Scenario: Es destructivo y se pregunta
- **WHEN** se restaura de forma interactiva
- **THEN** se pide una confirmación explícita que nombre el proyecto antes de tocar nada

#### Scenario: No confirmar no destruye
- **WHEN** no se confirma
- **THEN** la base de datos queda intacta

#### Scenario: Sin preguntar
- **WHEN** se indica que no se pregunte
- **THEN** la restauración se realiza sin interacción

#### Scenario: Una copia que no existe
- **WHEN** se nombra una copia inexistente
- **THEN** el comando falla nombrando las que sí existen

### Requirement: Las copias sobreviven al entorno

Las copias SHALL conservarse cuando el entorno se destruye.

#### Scenario: Tras destruir el entorno
- **WHEN** se destruye el entorno con sus volúmenes
- **THEN** las copias siguen estando disponibles para restaurarlas después

#### Scenario: Fuera del proyecto
- **WHEN** se crea una copia
- **THEN** no se escribe ningún fichero dentro del directorio del proyecto

### Requirement: Borrar una copia

`hm db remove` SHALL eliminar una copia del proyecto actual.

#### Scenario: Borrado
- **WHEN** se borra una copia existente
- **THEN** deja de aparecer en la lista y su espacio se libera

#### Scenario: Una que no existe
- **WHEN** se nombra una copia inexistente
- **THEN** el comando falla y no borra nada
