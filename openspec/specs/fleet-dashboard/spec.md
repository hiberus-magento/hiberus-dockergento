# fleet-dashboard Specification

## Purpose
TBD - created by archiving change terminal-dashboard. Update Purpose after archive.
## Requirements
### Requirement: Vista de la flota

`hm tui` SHALL mostrar los entornos Dockergento de la máquina con su estado.

#### Scenario: Entornos listados
- **WHEN** se abre el panel en una máquina con varios entornos
- **THEN** se muestra una fila por entorno con su nombre, su estado, cuántos servicios corren de cuántos, su rama y su ruta

#### Scenario: Worktrees distinguidos
- **WHEN** alguno de los entornos se levantó desde un worktree
- **THEN** aparece identificado como tal

#### Scenario: Entornos huérfanos
- **WHEN** algún entorno apunta a un directorio que ya no existe
- **THEN** aparece marcado

#### Scenario: Avisos del diagnóstico
- **WHEN** el diagnóstico global encuentra problemas
- **THEN** se muestran en la parte superior del panel

#### Scenario: Máquina sin entornos
- **WHEN** no hay ningún entorno en la máquina
- **THEN** el panel lo dice y explica cómo crear uno

### Requirement: Respuesta inmediata

El panel SHALL dibujarse antes de tener los datos.

#### Scenario: Apertura
- **WHEN** se abre el panel
- **THEN** aparece de inmediato con una indicación de que está cargando
- **AND** los datos se muestran cuando están disponibles

#### Scenario: Antigüedad de los datos
- **THEN** el panel indica de cuándo son los datos que muestra

#### Scenario: Refresco explícito
- **WHEN** el usuario pulsa la tecla de refresco
- **THEN** los datos se vuelven a pedir

#### Scenario: Sin refresco automático
- **WHEN** el panel permanece abierto sin que el usuario interactúe
- **THEN** no se vuelve a consultar Docker por su cuenta

### Requirement: Detalle de un entorno

El panel SHALL permitir abrir un entorno y ver lo que lo define.

#### Scenario: Apertura del detalle
- **WHEN** el usuario abre un entorno de la lista
- **THEN** se muestran sus URLs, su versión de Magento y el estado de cada servicio

#### Scenario: Vuelta a la flota
- **WHEN** el usuario vuelve atrás
- **THEN** regresa a la vista de flota

### Requirement: Acciones a través de la CLI

Las acciones del panel SHALL ejecutarse invocando los comandos de la CLI, no hablando con
Docker directamente.

#### Scenario: Arrancar un entorno
- **WHEN** el usuario pide arrancar el entorno seleccionado
- **THEN** se ejecuta el comando de arranque de la CLI sobre ese entorno

#### Scenario: Salida completa de la acción
- **WHEN** una acción se ejecuta
- **THEN** el panel deja el terminal a esa acción y su salida se ve entera
- **AND** al terminar se vuelve al panel

#### Scenario: Acción fallida
- **WHEN** una acción falla
- **THEN** el usuario ve el error completo, no una versión recortada

#### Scenario: Protecciones heredadas
- **WHEN** una acción está prohibida por una protección de la CLI, como ejecutarla desde un worktree
- **THEN** el panel obtiene el mismo rechazo que la CLI, sin saltárselo

#### Scenario: Nada destructivo
- **WHEN** se revisan las acciones disponibles
- **THEN** ninguna elimina volúmenes ni datos

### Requirement: Navegación descubrible

El panel SHALL indicar en pantalla qué teclas están disponibles.

#### Scenario: Teclas visibles
- **WHEN** el panel está en cualquiera de sus vistas
- **THEN** la última línea muestra las teclas disponibles en ese contexto

#### Scenario: Ayuda de teclas
- **WHEN** el usuario pide la ayuda
- **THEN** se muestran todas las teclas con lo que hacen

#### Scenario: Salir
- **WHEN** el usuario pulsa la tecla de salida o interrumpe con Ctrl-C
- **THEN** el panel termina

### Requirement: El terminal queda como estaba

Al terminar, el panel SHALL devolver el terminal a su estado anterior.

#### Scenario: Salida normal
- **WHEN** el usuario sale del panel
- **THEN** el terminal muestra el contenido que tenía antes de abrirlo, con el cursor visible

#### Scenario: Interrupción
- **WHEN** el usuario interrumpe el panel con Ctrl-C
- **THEN** el terminal queda igualmente restaurado

#### Scenario: Error inesperado
- **WHEN** el panel muere por un error
- **THEN** el terminal queda igualmente restaurado

#### Scenario: Redimensionado
- **WHEN** la ventana cambia de tamaño con el panel abierto
- **THEN** el panel se redibuja al tamaño nuevo

### Requirement: Sin terminal no hay panel

`hm tui` SHALL negarse a dibujar cuando la salida no es un terminal.

#### Scenario: Salida canalizada
- **WHEN** se ejecuta `hm tui` con la salida canalizada o redirigida
- **THEN** no se emite ninguna secuencia de control
- **AND** el comando falla explicando que necesita un terminal y sugiriendo la alternativa en JSON

