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

### Requirement: Fotogramas sin parpadeo

El panel SHALL redibujar sin que el terminal muestre en ningún momento una pantalla vacía o un
fotograma incompleto.

#### Scenario: La pantalla no se borra entre fotogramas
- **WHEN** el panel redibuja tras una pulsación de tecla
- **THEN** no emite la secuencia de borrado de pantalla completa, y sobrescribe cada línea
  borrando únicamente lo que quedara a su derecha

#### Scenario: Lo que sobra por debajo se limpia
- **WHEN** un fotograma es más corto que el anterior, por ejemplo al volver del detalle a una
  flota con pocos entornos
- **THEN** no queda ninguna línea del fotograma anterior visible por debajo

#### Scenario: El terminal no muestra fotogramas a medias
- **WHEN** se emite un fotograma
- **THEN** va delimitado por las marcas de salida sincronizada, de modo que un terminal que las
  soporte lo presente completo

#### Scenario: Un terminal que no conoce la salida sincronizada
- **WHEN** el terminal ignora esas marcas
- **THEN** el panel se dibuja correctamente igualmente, sin caracteres visibles de más

### Requirement: Respuesta inmediata a las teclas

El panel SHALL responder a una tecla sin recalcular el contenido desde los datos de origen.

#### Scenario: El layout se calcula al cargar, no al pulsar
- **WHEN** se pulsa una tecla que no cambia los datos, como moverse por la lista o cambiar de
  vista
- **THEN** el panel no ejecuta ningún proceso externo para redibujar

#### Scenario: Presupuesto de fotograma
- **WHEN** se compone y se pinta un fotograma con veinte entornos
- **THEN** el coste está por debajo del presupuesto declarado para un fotograma

#### Scenario: Un fotograma es una escritura
- **WHEN** se pinta un fotograma
- **THEN** se emite completo de una vez, no línea a línea

#### Scenario: Una flota más alta que el terminal
- **WHEN** hay más entornos que filas disponibles
- **THEN** la selección permanece visible y el panel indica qué parte de la lista se está
  mostrando, sin que el fotograma sobrepase la altura del terminal

### Requirement: El tamaño del terminal es una entrada de la composición

El panel SHALL recomponer el contenido cuando cambia el tamaño del terminal, y sólo entonces.

#### Scenario: Redimensionar
- **WHEN** se cambia el tamaño de la ventana
- **THEN** el panel recompone al nuevo tamaño y repinta inmediatamente, sin esperar a que se
  pulse una tecla y sin recargar los datos

#### Scenario: Redimensionar no cierra el panel
- **WHEN** la señal de redimensionado llega mientras el panel espera una tecla
- **THEN** el panel sigue esperando esa tecla después de haber repintado

#### Scenario: El pie se adapta al ancho
- **WHEN** el ancho no da para la lista larga de teclas
- **THEN** se muestra la forma corta completa, en lugar de la larga recortada

