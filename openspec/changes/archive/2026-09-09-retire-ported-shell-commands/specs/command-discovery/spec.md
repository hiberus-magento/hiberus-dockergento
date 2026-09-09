# Delta for command-discovery

## MODIFIED Requirements

### Requirement: Comandos agrupados por propósito

`hm --help` SHALL presentar los comandos agrupados por propósito, en un orden declarado
fuera del código.

#### Scenario: Grupos visibles
- **WHEN** se ejecuta `hm --help`
- **THEN** los comandos aparecen bajo cabeceras de grupo y no en una única lista alfabética

#### Scenario: El orden lo fijan los datos
- **WHEN** se cambia el orden de los grupos en la configuración
- **THEN** la ayuda los presenta en el nuevo orden, sin tocar código

#### Scenario: Comando sin grupo declarado
- **WHEN** un comando no declara grupo
- **THEN** aparece igualmente, en un grupo final para los no clasificados

#### Scenario: Comandos personalizados del proyecto
- **WHEN** un proyecto tiene comandos propios
- **THEN** siguen apareciendo en su sección

#### Scenario: Ningún comando desaparece
- **WHEN** se comparan los comandos listados con los comandos declarados en
  `data/command_descriptions.json`
- **THEN** están todos, tenga o no cada uno un fichero bajo `console/commands/`
(Previously: comparaba los comandos listados contra los ficheros de comandos existentes; un
comando solo-Go no tenía fichero y quedaba fuera de esa comparación.)

#### Scenario: El orden no cambia
- **WHEN** se compara la salida de `hm --help` antes y después de que un comando deje de tener
  fichero propio
- **THEN** el orden en que aparecen los comandos es idéntico, byte a byte
