# domain-resolution Specification

## Purpose
TBD - created by archiving change resolve-domains-without-hosts. Update Purpose after archive.
## Requirements
### Requirement: No escribir en el sistema cuando no hace falta

La herramienta SHALL comprobar si el dominio del proyecto ya resuelve antes de modificar el fichero
de hosts del sistema.

#### Scenario: El dominio ya resuelve
- **WHEN** el dominio del proyecto ya resuelve a una dirección local
- **THEN** no se modifica el fichero de hosts ni se pide la contraseña del sistema

#### Scenario: El dominio no resuelve
- **WHEN** el dominio no resuelve
- **THEN** se añade al fichero de hosts como hasta ahora

#### Scenario: Un dominio que resuelve a internet
- **WHEN** el dominio resuelve a una dirección que no es local
- **THEN** se añade al fichero de hosts, porque el objetivo es precisamente trabajarlo en local

#### Scenario: Ya estaba en el fichero
- **WHEN** el dominio ya figura en el fichero de hosts
- **THEN** no se duplica

### Requirement: El diagnóstico dice qué está resolviendo

El diagnóstico SHALL informar de si el dominio del proyecto resuelve y por qué mecanismo.

#### Scenario: Se informa del mecanismo
- **WHEN** se diagnostica un proyecto
- **THEN** se indica si su dominio resuelve, y si lo hace por el fichero de hosts o por DNS

#### Scenario: Un dominio que no resuelve de ninguna forma
- **WHEN** el dominio del proyecto no resuelve
- **THEN** el diagnóstico lo señala y dice cómo arreglarlo

### Requirement: Nothing is added that buys nothing

The tool SHALL NOT write an entry for a name that already resolves to this machine.

#### Scenario: A wildcard resolver for the TLD

- **WHEN** the name already resolves to a loopback address
- **THEN** the hosts file is left alone, and it is said

#### Scenario: The same name twice

- **WHEN** the hosts file already resolves the name, whoever wrote the line
- **THEN** no second line is added

### Requirement: Only what this tool wrote is this tool's to remove

The tool SHALL leave alone every line it did not write.

#### Scenario: A line somebody wrote by hand

- **WHEN** an entry with no marker is asked to be removed
- **THEN** nothing is removed, and it is said

#### Scenario: Removing one it wrote

- **WHEN** an entry this tool added is removed
- **THEN** it is gone and the rest of the file is exactly as it was
