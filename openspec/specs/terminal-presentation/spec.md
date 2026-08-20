# terminal-presentation Specification

## Purpose
TBD - created by archiving change polish-terminal-ux. Update Purpose after archive.
## Requirements
### Requirement: Credenciales sin eco

Ninguna pregunta que pida una credencial SHALL mostrar en pantalla lo que el usuario escribe.

#### Scenario: Contraseña de base de datos remota
- **WHEN** se pregunta la contraseña de la base de datos en `hm transfer-db`
- **THEN** los caracteres escritos no aparecen en pantalla
- **AND** la contraseña no queda en el historial visible del terminal

#### Scenario: El flujo continúa legible
- **WHEN** el usuario confirma la contraseña con Enter
- **THEN** el mensaje siguiente empieza en una línea nueva

#### Scenario: Credenciales pedidas explícitamente
- **WHEN** se ejecuta `hm describe --with-secrets`
- **THEN** las credenciales se muestran, porque se han pedido

### Requirement: Decisión de color según el estándar del ecosistema

La CLI SHALL decidir si colorea su salida atendiendo, por orden de prioridad, a la petición
explícita del usuario, a las variables de entorno del estándar y a si stdout es un terminal.

#### Scenario: Petición explícita de no colorear
- **WHEN** se ejecuta cualquier comando con `--no-color`
- **THEN** la salida no contiene secuencias de color

#### Scenario: NO_COLOR definida
- **WHEN** `NO_COLOR` está definida y no vacía
- **THEN** la salida no contiene secuencias de color

#### Scenario: Terminal sin capacidades
- **WHEN** `TERM` vale `dumb` o está vacía
- **THEN** la salida no contiene secuencias de color

#### Scenario: Color forzado en una tubería
- **WHEN** `FORCE_COLOR` está definida y la salida está canalizada
- **THEN** la salida sí contiene secuencias de color

#### Scenario: La petición explícita gana al forzado
- **WHEN** `FORCE_COLOR` está definida y además se pasa `--no-color`
- **THEN** la salida no contiene secuencias de color

#### Scenario: Terminal interactivo sin variables
- **WHEN** stdout es un terminal y no hay ninguna de esas variables definidas
- **THEN** la salida contiene color, como hasta ahora

#### Scenario: El contenido no depende del color
- **WHEN** se compara la salida de un mismo comando con color y sin color
- **THEN** el texto es el mismo, sin las secuencias de color

### Requirement: Preguntar no destruye el contexto

El flujo de preguntas NO SHALL borrar el contenido que ya hay en el terminal.

#### Scenario: Varias preguntas seguidas
- **WHEN** un comando hace varias preguntas seguidas, como `hm setup`
- **THEN** las preguntas anteriores y sus respuestas siguen visibles

#### Scenario: Selección entre opciones
- **WHEN** se muestra un selector de opciones
- **THEN** lo que había en pantalla antes sigue visible

#### Scenario: Tabla redibujada
- **WHEN** el usuario edita una versión y la tabla de requisitos se muestra de nuevo
- **THEN** la tabla anterior sigue visible, separada de la nueva

