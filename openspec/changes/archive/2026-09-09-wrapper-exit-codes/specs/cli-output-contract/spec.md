# Delta for cli-output-contract

## MODIFIED Requirements

### Requirement: Errores estructurados

En modo JSON, los errores SHALL emitirse por stderr como un objeto con `ok: false` y un
objeto `error` que contenga `code`, `type`, `message` y `hint`.

#### Scenario: Docker parado en modo JSON
- **WHEN** se ejecuta un comando que necesita Docker, el demonio no está disponible y el modo es JSON
- **THEN** stderr contiene un objeto JSON con `ok: false` y `error.type` igual a `docker_unavailable`
- **AND** stdout no contiene el error
- **AND** el código de salida es 3

#### Scenario: Error en modo texto
- **WHEN** ocurre el mismo error con el modo de salida en texto
- **THEN** se muestra el mensaje de error legible actual
- **AND** el código de salida es igualmente 3

#### Scenario: Falla el programa que un envoltorio ejecuta
- **WHEN** un envoltorio que no está en la exclusión de paso (`varnish-on`, `varnish-off`) ejecuta un programa dentro del contenedor y ese programa termina con un estado distinto de cero, sin error de Go
- **THEN** en modo JSON stderr contiene exactamente un objeto con `ok: false` y `error.code` igual a ese estado, y un mensaje que nombra el paso que falló
- **AND** stdout no contiene documento de éxito
- **AND** en modo texto se muestra la salida del propio contenedor más una línea legible que nombra el paso, con el mismo código de salida

Prueba: `TestAFailedVCLEditReportsTheContainersCode`, `TestAFailedCacheToggleStopsBeforeThePurge`
(nuevas).
(Antes: los dos guardas de `varnish` no emitían objeto ni mensaje en este camino — un estado
distinto de cero sin error de Go no producía nada por stderr.)

### Requirement: Códigos de salida estables

La CLI SHALL usar códigos de salida específicos y documentados en lugar de un `exit 1`
genérico.

#### Scenario: Tabla de códigos
- **WHEN** un comando termina
- **THEN** devuelve 0 si tuvo éxito, 2 si los argumentos eran inválidos, 3 si Docker no estaba disponible, 4 si el proyecto no estaba configurado, 5 si un servicio requerido no estaba levantado, y 1 para cualquier otro error

#### Scenario: Servicio no levantado
- **WHEN** se ejecuta un comando que requiere el servicio `db` y ese servicio no está corriendo en el proyecto actual
- **THEN** el código de salida es 5

#### Scenario: Documentación de los códigos
- **WHEN** se consulta la documentación de la CLI
- **THEN** la tabla completa de códigos de salida está publicada

#### Scenario: El código de un envoltorio es el del programa, no el de la tabla
- **WHEN** el programa que `varnish-on` o `varnish-off` ejecuta dentro del contenedor falla con un estado arbitrario distinto de cero
- **THEN** la herramienta termina con ese mismo estado, que no tiene por qué estar en la tabla 0/1/2/3/4/5 de arriba: esa tabla gobierna los fallos que decide la herramienta, no los del programa que corre dentro

Prueba: `TestAFailedVCLEditReportsTheContainersCode`, `TestAFailedCacheToggleStopsBeforeThePurge`
(nuevas).
(Antes: en este camino la herramienta respondía 0 sin mirar el estado del contenedor.)
