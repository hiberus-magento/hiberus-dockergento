# version-management Specification

## Purpose
TBD - created by archiving change version-switching. Update Purpose after archive.
## Requirements
### Requirement: Identificación exacta de la versión instalada

`hm --version` SHALL informar de la referencia exacta que se está ejecutando, no del último
tag redondeado.

#### Scenario: Ejecutando justo en una versión
- **WHEN** la instalación está en el commit de un tag
- **THEN** se informa de ese tag

#### Scenario: Ejecutando por delante de una versión
- **WHEN** la instalación está por delante del último tag
- **THEN** se informa del tag, de cuántos commits hay por encima y del identificador del commit

#### Scenario: Rama y estado
- **THEN** se informa también de la rama, o de que el checkout está desacoplado, y de si hay
  cambios sin guardar

#### Scenario: Ruta de instalación
- **THEN** se informa de dónde está instalada la herramienta

#### Scenario: Consumo programático
- **WHEN** se pide la versión en JSON
- **THEN** se obtiene un objeto con esos datos por separado

### Requirement: Cambio de versión

`hm switch` SHALL cambiar la instalación a la versión o rama indicada.

#### Scenario: Cambio a una versión
- **WHEN** se ejecuta `hm switch` con un tag existente
- **THEN** la instalación queda en ese tag
- **AND** se informa de la versión resultante

#### Scenario: Vuelta a la rama estable
- **WHEN** se ejecuta `hm switch --stable`
- **THEN** la instalación queda en la rama estable

#### Scenario: Listado de lo disponible
- **WHEN** se ejecuta `hm switch --list`
- **THEN** se muestran las versiones y las ramas disponibles, indicando la actual

#### Scenario: Referencia inexistente
- **WHEN** se pide cambiar a una referencia que no existe
- **THEN** el comando falla con un mensaje que sugiere `--list`
- **AND** la instalación no cambia

#### Scenario: Versiones nuevas visibles
- **WHEN** se han publicado versiones nuevas desde la última vez
- **THEN** `hm switch` las encuentra sin que el usuario tenga que refrescar nada a mano

#### Scenario: Autocompletado al día
- **WHEN** el cambio de versión se completa
- **THEN** el autocompletado se regenera para la versión nueva

### Requirement: El cambio de versión no destruye trabajo

`hm switch` NO SHALL descartar cambios locales del directorio de instalación.

#### Scenario: Cambios sin guardar
- **WHEN** el directorio de instalación tiene cambios sin guardar y se pide cambiar de versión
- **THEN** el comando se niega, indica qué ficheros están modificados y no cambia nada

#### Scenario: Instalación que no es un clon de git
- **WHEN** la instalación no es un clon de git
- **THEN** el comando lo explica en lugar de fallar de forma opaca

### Requirement: Actualizar no saca de la versión que se está validando

`hm update` NO SHALL modificar un checkout desacoplado.

#### Scenario: Actualizar estando en una versión
- **WHEN** se ejecuta `hm update` con la instalación en un tag
- **THEN** no se descarga ni se cambia nada
- **AND** el mensaje explica que se está en una versión concreta y remite a `hm switch`

#### Scenario: Actualizar estando en una rama
- **WHEN** se ejecuta `hm update` con la instalación en una rama
- **THEN** se actualiza como hasta ahora

### Requirement: Volver a una versión anterior es seguro

Cambiar a una versión anterior NO SHALL romper los proyectos existentes.

#### Scenario: Proyecto configurado con una versión posterior
- **WHEN** un proyecto tiene su configuración generada por una versión posterior y se baja la
  herramienta a una anterior
- **THEN** los comandos de esa versión anterior siguen funcionando sobre el proyecto

#### Scenario: Cachés de versiones posteriores
- **WHEN** existen cachés escritas por una versión posterior
- **THEN** la versión anterior las ignora sin fallar

