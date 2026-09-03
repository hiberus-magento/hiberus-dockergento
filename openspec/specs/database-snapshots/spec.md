# database-snapshots Specification

## Purpose
TBD - created by archiving change add-database-snapshots. Update Purpose after archive.
## Requirements
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

### Requirement: Vaciar las copias

`hm db clear` SHALL eliminar todas las copias del proyecto, o las de todos los proyectos, siempre
tras una confirmación explícita.

#### Scenario: Las de este proyecto
- **WHEN** se vacían las copias sin más indicación
- **THEN** se eliminan las del proyecto actual y ninguna de otro

#### Scenario: Las de todos
- **WHEN** se indica que el alcance es toda la máquina
- **THEN** se eliminan las copias de todos los proyectos

#### Scenario: Se pregunta antes, nombrando lo que se destruye
- **WHEN** se vacían copias de forma interactiva
- **THEN** se enumera lo que se va a borrar y se exige escribir el nombre de lo que se destruye

#### Scenario: No confirmar no borra
- **WHEN** no se confirma
- **THEN** ninguna copia se elimina

#### Scenario: Nada que borrar
- **WHEN** no hay ninguna copia
- **THEN** se informa y el comando termina correctamente

### Requirement: Compatibilidad con todas las versiones de base de datos

Las copias SHALL funcionar con cualquiera de las imágenes de base de datos que la herramienta puede
configurar.

#### Scenario: Herramientas renombradas
- **WHEN** la imagen sólo trae los nombres antiguos de cliente y volcador, o sólo los nuevos
- **THEN** se resuelven dentro del contenedor y el comando funciona igual

#### Scenario: Ida y vuelta en cada versión
- **WHEN** se copia y se restaura en cualquiera de las versiones soportadas
- **THEN** vuelven los datos, las rutinas y los disparadores, y no sobrevive nada creado después de
  la copia

### Requirement: Borrar una copia

`hm db remove` SHALL eliminar una copia del proyecto actual.

#### Scenario: Borrado
- **WHEN** se borra una copia existente
- **THEN** deja de aparecer en la lista y su espacio se libera

#### Scenario: Una que no existe
- **WHEN** se nombra una copia inexistente
- **THEN** el comando falla y no borra nada

### Requirement: A copy is usable or it is not there

The tool SHALL NOT leave an incomplete copy where a complete one is expected.

#### Scenario: The copy is interrupted

- **WHEN** taking a copy fails or is interrupted
- **THEN** no snapshot by that name is listed, and nothing half-written is left in its place

### Requirement: A copy holds the data and nothing else

The tool SHALL NOT write anything into a copy but what the database dumped.

#### Scenario: The dumper warns about something

- **WHEN** the command that writes the copy also writes a warning about itself
- **THEN** the warning is not in the copy

#### Scenario: A database large enough to take minutes

- **WHEN** copying takes longer than a query would
- **THEN** it is not cut off

### Requirement: Restoring replaces rather than merges

The tool SHALL empty the database before restoring a copy into it.

#### Scenario: Something was created after the copy was taken

- **WHEN** a copy is restored over a database that has changed since
- **THEN** what was created afterwards is gone, and the database is what the copy holds

#### Scenario: What the copy holds was never anonymised here

- **WHEN** a copy is restored
- **THEN** the record of the data having been anonymised is cleared

### Requirement: Destroying is confirmed by naming what is destroyed

The tool SHALL require the name of what is at stake, not a letter.

#### Scenario: Restoring

- **WHEN** a restore is confirmed with anything other than the project's name
- **THEN** nothing is restored

#### Scenario: Clearing every project

- **WHEN** clearing the copies of every project on the machine is confirmed with one project's
  name
- **THEN** nothing is deleted
