## ADDED Requirements

### Requirement: Batería de comprobaciones

`hm doctor` SHALL ejecutar un conjunto de comprobaciones sobre la máquina y sobre el
proyecto actual, y devolver el resultado de cada una.

#### Scenario: Todo correcto
- **WHEN** se ejecuta `hm doctor` en una máquina y un proyecto sin problemas
- **THEN** todas las comprobaciones aparecen como correctas
- **AND** el código de salida es 0

#### Scenario: Resultado con problemas
- **WHEN** alguna comprobación detecta un problema
- **THEN** se indica su severidad, qué ha detectado y una acción concreta para resolverlo

#### Scenario: Comprobación individual
- **WHEN** se ejecuta `hm doctor` pidiendo una única comprobación
- **THEN** sólo se ejecuta esa comprobación

### Requirement: Ámbito global y de proyecto

`hm doctor` SHALL distinguir las comprobaciones de la máquina de las del proyecto, y SHALL
poder ejecutarse fuera de un proyecto.

#### Scenario: Fuera de un proyecto
- **WHEN** se ejecuta `hm doctor` en un directorio que no es un proyecto Dockergento
- **THEN** se ejecutan sólo las comprobaciones globales
- **AND** se indica que las de proyecto se han omitido
- **AND** el comando no falla por ese motivo

#### Scenario: Dentro de un proyecto
- **WHEN** se ejecuta `hm doctor` dentro de un proyecto
- **THEN** se ejecutan las comprobaciones globales y las del proyecto

### Requirement: Severidad y código de salida

Las comprobaciones SHALL clasificarse en correcto, aviso y error, y sólo los errores SHALL
provocar un código de salida distinto de cero.

#### Scenario: Sólo avisos
- **WHEN** el diagnóstico termina con avisos pero sin errores
- **THEN** el código de salida es 0

#### Scenario: Algún error
- **WHEN** al menos una comprobación termina en error
- **THEN** el código de salida es distinto de cero

#### Scenario: Encadenado en un script
- **WHEN** se ejecuta `hm doctor && hm start`
- **THEN** `hm start` se ejecuta si no hubo errores y no se ejecuta si los hubo

### Requirement: Diagnóstico de puertos ocupados

El diagnóstico SHALL comprobar los puertos que el entorno necesita publicar e identificar
qué los ocupa cuando no están libres.

#### Scenario: Puerto ocupado por otro entorno Dockergento
- **WHEN** un puerto requerido está ocupado por un contenedor de otro proyecto Dockergento
- **THEN** se indica el nombre de ese proyecto
- **AND** la acción propuesta es detener ese entorno

#### Scenario: Puerto ocupado por un proceso del host
- **WHEN** un puerto requerido está ocupado por un proceso que no es un contenedor
- **THEN** se indica el proceso si el sistema permite averiguarlo sin privilegios

#### Scenario: Herramienta de inspección no disponible
- **WHEN** no hay ninguna herramienta disponible para averiguar qué ocupa el puerto
- **THEN** la comprobación devuelve un aviso explicando la limitación
- **AND** no se solicitan privilegios de administrador

### Requirement: Robustez del diagnóstico

Ninguna comprobación SHALL impedir que se ejecuten las demás, y el diagnóstico completo
SHALL terminar en un tiempo acotado.

#### Scenario: Comprobación que se cuelga
- **WHEN** una comprobación supera su límite de tiempo
- **THEN** se reporta como aviso indicando que expiró
- **AND** el resto de comprobaciones se ejecutan igualmente

#### Scenario: Comprobación que falla de forma inesperada
- **WHEN** una comprobación falla por un error no previsto
- **THEN** se reporta como aviso con el motivo
- **AND** el diagnóstico continúa

#### Scenario: Docker no disponible
- **WHEN** el demonio de Docker no está corriendo
- **THEN** se indica como error con la acción de arrancarlo
- **AND** las comprobaciones que dependen de Docker se omiten indicando el motivo

### Requirement: Diagnóstico sin efectos secundarios

`hm doctor` SHALL limitarse a diagnosticar y no SHALL modificar el sistema ni el proyecto.

#### Scenario: Falta una entrada de dominio
- **WHEN** el dominio del proyecto no resuelve a local
- **THEN** se propone la orden que lo arregla
- **AND** no se modifica ningún fichero del sistema

#### Scenario: Configuración de Compose inválida
- **WHEN** la configuración de Compose del proyecto no es válida
- **THEN** se reporta el error con el detalle devuelto por Compose
- **AND** no se regenera ningún fichero
