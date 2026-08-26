# environment-sharing Specification

## Purpose
TBD - created by archiving change add-share-command. Update Purpose after archive.
## Requirements
### Requirement: Exponer el proyecto temporalmente

`hm share` SHALL proporcionar una dirección pública temporal que sirva el proyecto actual.

#### Scenario: Se obtiene una dirección
- **WHEN** se comparte el proyecto
- **THEN** se informa de una dirección pública desde la que el proyecto es accesible

#### Scenario: Sirve el contenido del proyecto
- **WHEN** alguien accede a esa dirección
- **THEN** recibe lo que sirve el proyecto, no un error del túnel

#### Scenario: Funciona con y sin proxy
- **WHEN** el proyecto publica puertos, o no publica ninguno por usar el proxy
- **THEN** compartir funciona igual en ambos casos

### Requirement: Exponer es una decisión consciente

`hm share` SHALL advertir de lo que implica y pedir confirmación antes de abrir nada.

#### Scenario: Se advierte antes
- **WHEN** se pide compartir de forma interactiva
- **THEN** se indica que cualquiera con la dirección accede al entorno, antes de abrirlo

#### Scenario: No confirmar no abre
- **WHEN** no se confirma
- **THEN** no se abre ningún túnel

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se abre sin preguntar

### Requirement: Cerrar no deja nada

`hm share` SHALL cerrar el túnel y eliminar lo que haya creado cuando termina.

#### Scenario: Al terminar el comando
- **WHEN** se interrumpe el comando
- **THEN** el túnel se cierra y no queda ningún contenedor suyo

#### Scenario: Cierre explícito
- **WHEN** se pide cerrar lo que este proyecto dejó abierto
- **THEN** se cierra, y se informa si no había nada

#### Scenario: Uno nuevo recoge el anterior
- **WHEN** se comparte de nuevo habiendo quedado un túnel de antes
- **THEN** el anterior se cierra antes de abrir el nuevo

