## ADDED Requirements

### Requirement: Registros del entorno

`hm logs` SHALL mostrar los registros de los contenedores del proyecto.

#### Scenario: Todo el proyecto
- **WHEN** se ejecuta sin argumentos dentro de un proyecto
- **THEN** se muestran los registros de todos sus servicios

#### Scenario: Un servicio concreto
- **WHEN** se nombra uno o varios servicios
- **THEN** se muestran únicamente los registros de esos servicios

#### Scenario: Opciones de Compose
- **WHEN** se pasan opciones como seguimiento en vivo, número de líneas o marca temporal
- **THEN** llegan intactas a Compose, sin que el router las interprete

#### Scenario: Un servicio que no existe
- **WHEN** se nombra un servicio que el proyecto no tiene
- **THEN** el comando falla con el código de salida de servicio y nombra los servicios
  disponibles

#### Scenario: La salida son datos
- **WHEN** la salida se canaliza o se pide JSON
- **THEN** los registros salen tal cual, sin envolverse en ninguna estructura

### Requirement: Abrir el entorno

`hm launch` SHALL abrir en el navegador una dirección del entorno actual.

#### Scenario: La tienda por defecto
- **WHEN** se ejecuta sin opciones en un terminal
- **THEN** se abre la dirección pública del proyecto y se informa de qué se ha abierto

#### Scenario: Otros destinos
- **WHEN** se pide el panel de administración, el correo, la cola de mensajes o el buscador
- **THEN** se abre la dirección correspondiente de las que el proyecto publica

#### Scenario: Dos destinos a la vez
- **WHEN** se piden dos destinos en la misma invocación
- **THEN** el comando falla como error de uso en lugar de abrir varias pestañas

#### Scenario: Sin sitio donde abrir
- **WHEN** la salida no es un terminal, se pide JSON, o el sistema no tiene con qué abrir un
  navegador
- **THEN** se escribe la dirección y no se abre nada

#### Scenario: Un proyecto sin dominio
- **WHEN** el proyecto no tiene dominio configurado
- **THEN** el comando lo dice, en lugar de abrir una dirección inventada

#### Scenario: No arranca lo que está parado
- **WHEN** el entorno está detenido
- **THEN** el comando abre o escribe la dirección sin arrancar nada
