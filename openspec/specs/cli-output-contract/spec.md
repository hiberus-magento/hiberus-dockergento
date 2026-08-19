# cli-output-contract Specification

## Purpose
TBD - created by archiving change add-cli-output-contract. Update Purpose after archive.
## Requirements
### Requirement: Selección del formato de salida

La CLI SHALL emitir su salida en texto o en JSON según el flag recibido y, en su ausencia,
según si stdout está conectado a un terminal.

#### Scenario: Persona en un terminal
- **WHEN** se ejecuta un comando informativo con stdout conectado a un TTY y sin flags de formato
- **THEN** la salida es el texto legible actual, con colores

#### Scenario: Salida canalizada
- **WHEN** se ejecuta un comando informativo con stdout no conectado a un TTY y sin flags de formato
- **THEN** la salida es un objeto JSON válido y no contiene códigos de color ANSI

#### Scenario: JSON forzado en terminal
- **WHEN** se ejecuta un comando informativo con `--json` desde un terminal
- **THEN** la salida es un objeto JSON válido

#### Scenario: Texto forzado al canalizar
- **WHEN** se ejecuta un comando informativo con `--no-json` y stdout canalizado
- **THEN** la salida es texto legible

### Requirement: Envoltura de las respuestas JSON

Toda salida JSON de éxito SHALL incluir `schema_version`, `command`, `ok: true` y un objeto
`data`, y SHALL escribirse en stdout.

#### Scenario: Respuesta de éxito
- **WHEN** un comando informativo termina correctamente en modo JSON
- **THEN** stdout contiene un único objeto JSON con las claves `schema_version`, `command`, `ok` y `data`
- **AND** `ok` es `true`
- **AND** el objeto es parseable por `jq` sin errores

#### Scenario: Valores con caracteres especiales
- **WHEN** un valor a emitir contiene comillas, saltos de línea o caracteres UTF-8
- **THEN** el JSON resultante sigue siendo válido y el valor se recupera intacto al parsearlo

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

### Requirement: Modo no interactivo

Con `--yes` o con `HM_NON_INTERACTIVE=1`, la CLI SHALL completar su ejecución sin esperar
entrada del usuario en ningún momento.

#### Scenario: Pregunta con valor por defecto
- **WHEN** un comando llegaría a una pregunta que tiene valor por defecto y el modo no interactivo está activo
- **THEN** se usa el valor por defecto sin mostrar la pregunta

#### Scenario: Pregunta sin valor por defecto
- **WHEN** un comando llegaría a una pregunta sin valor por defecto razonable y el modo no interactivo está activo
- **THEN** el comando falla con código 2
- **AND** el mensaje de error indica qué flag hay que pasar para evitar la pregunta

#### Scenario: Ejecución sin terminal ni entrada
- **WHEN** cualquier comando se ejecuta con stdin cerrado, sin TTY y con `HM_NON_INTERACTIVE=1`
- **THEN** el comando termina y no queda bloqueado esperando entrada

#### Scenario: Compatibilidad con la variable existente
- **WHEN** se ejecuta `setup` o `install` con `USE_DEFAULT_SETTINGS` ya definida
- **THEN** el comportamiento actual se mantiene

### Requirement: Comandos de flujo de datos y de paso

Los comandos cuya salida estándar es un dato o la salida de un proceso hijo SHALL quedar
excluidos de la envoltura JSON.

#### Scenario: Volcado de base de datos
- **WHEN** se ejecuta `hm mysqldump` y se redirige su salida a un fichero
- **THEN** el fichero contiene SQL y no un objeto JSON

#### Scenario: Consulta a la base de datos sin terminal
- **WHEN** se ejecuta `hm mysql -q "SELECT 1"` desde un proceso sin TTY
- **THEN** se devuelve el resultado de la consulta y no una envoltura JSON

#### Scenario: Comando de paso
- **WHEN** se ejecuta `hm magento cache:clean` desde un proceso sin TTY
- **THEN** la salida es la del comando de Magento, sin envolver

#### Scenario: Mensajes de progreso
- **WHEN** un comando de flujo de datos emite mensajes de progreso
- **THEN** esos mensajes van a stderr y no contaminan stdout

