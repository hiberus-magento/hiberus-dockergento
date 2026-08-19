# environment-introspection Specification

## Purpose
TBD - created by archiving change add-describe-and-list-commands. Update Purpose after archive.
## Requirements
### Requirement: Descripción del entorno actual

`hm describe` SHALL devolver la información que identifica y configura el proyecto del
directorio actual.

#### Scenario: Proyecto levantado
- **WHEN** se ejecuta `hm describe` dentro de un proyecto con sus contenedores arriba
- **THEN** se devuelve el nombre del proyecto, su dominio y URLs, la versión de Magento, la versión y el estado de cada servicio, las rutas relevantes y el estado de Xdebug

#### Scenario: Proyecto parado
- **WHEN** se ejecuta `hm describe` dentro de un proyecto con los contenedores parados
- **THEN** se devuelve toda la información que proviene de ficheros de configuración
- **AND** el estado del entorno se indica como parado
- **AND** el comando termina con éxito

#### Scenario: Fuera de un proyecto
- **WHEN** se ejecuta `hm describe` en un directorio que no es un proyecto Dockergento
- **THEN** el comando falla con el código de salida de proyecto no configurado
- **AND** el mensaje indica cómo crear o configurar el proyecto

#### Scenario: Versión de Magento indeterminable
- **WHEN** el proyecto no tiene `composer.lock`
- **THEN** la versión de Magento se informa como desconocida
- **AND** el resto de la información se devuelve igualmente

### Requirement: Protección de credenciales

`hm describe` SHALL omitir las credenciales de su salida salvo que se soliciten
explícitamente.

#### Scenario: Salida por defecto
- **WHEN** se ejecuta `hm describe` sin flags adicionales
- **THEN** la salida no contiene contraseñas ni secretos

#### Scenario: Credenciales solicitadas
- **WHEN** se ejecuta `hm describe --with-secrets`
- **THEN** la salida incluye las credenciales de base de datos y de los servicios que las tengan

### Requirement: Inventario de entornos de la máquina

`hm list` SHALL enumerar los entornos Dockergento presentes en la máquina, con
independencia del directorio desde el que se ejecute.

#### Scenario: Ejecución fuera de cualquier proyecto
- **WHEN** se ejecuta `hm list` desde un directorio que no es un proyecto
- **THEN** se devuelve el inventario de entornos igualmente

#### Scenario: Varios entornos
- **WHEN** hay varios entornos Dockergento en la máquina, unos levantados y otros parados
- **THEN** cada uno aparece una sola vez, con su estado, su ruta de origen y su dominio

#### Scenario: Distinción de worktrees
- **WHEN** un entorno se levantó desde un worktree
- **THEN** aparece identificado como worktree y asociado al proyecto del que procede

#### Scenario: Entorno huérfano
- **WHEN** un entorno apunta a un directorio de origen que ya no existe
- **THEN** aparece marcado como huérfano

#### Scenario: Máquina sin entornos
- **WHEN** no hay ningún entorno Dockergento en la máquina
- **THEN** el comando termina con éxito e indica que no hay entornos

### Requirement: Formato de la información expuesta

La salida JSON de ambos comandos SHALL seguir un esquema documentado y versionado.

#### Scenario: Esquema versionado
- **WHEN** se obtiene la salida en JSON de cualquiera de los dos comandos
- **THEN** incluye `schema_version`
- **AND** las claves documentadas como estables están presentes aunque su valor sea nulo o vacío

#### Scenario: Consumo programático
- **WHEN** un proceso sin terminal ejecuta `hm describe` y canaliza su salida
- **THEN** obtiene JSON válido sin códigos de color

#### Scenario: Consumo humano
- **WHEN** una persona ejecuta `hm describe` en un terminal
- **THEN** obtiene una salida agrupada por bloques, con las URLs y el estado en primer lugar

