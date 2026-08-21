## ADDED Requirements

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
