## ADDED Requirements

### Requirement: Comprobar el código con lo que el proyecto tenga

`hm verify` SHALL ejecutar las comprobaciones de calidad disponibles en el proyecto e informar del
resultado de cada una.

#### Scenario: Un proyecto con herramientas
- **WHEN** el proyecto tiene herramientas de análisis instaladas
- **THEN** se ejecutan y se informa del resultado de cada una

#### Scenario: Un proyecto sin ninguna
- **WHEN** el proyecto no tiene ninguna instalada
- **THEN** se informa de que no hay nada que ejecutar y el comando no falla por ello

#### Scenario: Lo ausente no es un fallo
- **WHEN** una herramienta no está instalada
- **THEN** su comprobación se informa como omitida, indicando el motivo, y no cuenta como error

#### Scenario: La sintaxis siempre se comprueba
- **WHEN** se verifica cualquier proyecto, tenga lo que tenga instalado
- **THEN** se comprueba que los ficheros PHP son sintácticamente válidos

#### Scenario: Un error de sintaxis se detecta
- **WHEN** un fichero PHP del proyecto no es válido
- **THEN** la verificación falla e indica el fichero

### Requirement: Alcance de la verificación

`hm verify` SHALL permitir acotar qué se comprueba, y no ejecutar por defecto las comprobaciones que tardan minutos.

#### Scenario: Sólo lo cambiado
- **WHEN** se pide verificar únicamente lo modificado
- **THEN** las comprobaciones se limitan a los ficheros que difieren de la rama base

#### Scenario: Sin forma de saber qué cambió
- **WHEN** no se puede determinar la rama base
- **THEN** se verifica todo y se indica que se ha hecho así

#### Scenario: Las comprobaciones lentas van aparte
- **WHEN** se verifica sin pedir nada más
- **THEN** no se ejecutan las comprobaciones que tardan minutos, y se puede pedirlas explícitamente

### Requirement: Un resultado que se puede encadenar

`hm verify` SHALL terminar con éxito sólo cuando ninguna comprobación haya encontrado problemas.

#### Scenario: Todo correcto
- **WHEN** ninguna comprobación encuentra problemas
- **THEN** el comando termina con éxito

#### Scenario: Algo falla
- **WHEN** alguna comprobación encuentra problemas
- **THEN** el comando termina con error

#### Scenario: Para una máquina
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** cada comprobación aparece con su estado, su recuento de problemas y su salida
