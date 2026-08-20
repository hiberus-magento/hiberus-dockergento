## ADDED Requirements

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
- **WHEN** se comparan los comandos listados con los ficheros de comandos existentes
- **THEN** están todos

### Requirement: Uso y ejemplos

La ayuda SHALL empezar por una línea de uso y SHALL incluir ejemplos de las tareas más
frecuentes.

#### Scenario: Línea de uso
- **WHEN** se ejecuta `hm --help`
- **THEN** la primera línea de contenido indica cómo se invoca la herramienta

#### Scenario: Ejemplos
- **THEN** se muestran ejemplos concretos con lo que hace cada uno

#### Scenario: Los ejemplos se declaran fuera del código
- **WHEN** se cambian los ejemplos en la configuración
- **THEN** la ayuda muestra los nuevos

#### Scenario: Pie con las opciones globales
- **THEN** se muestran las opciones globales y cómo consultar la ayuda de un comando concreto

### Requirement: Identidad visual proporcionada

La cabecera SHALL identificar la herramienta sin desplazar el contenido, y SHALL degradar
en terminales que no puedan dibujarla.

#### Scenario: Terminal con UTF-8
- **WHEN** la configuración regional indica UTF-8 y la salida es un terminal
- **THEN** se dibuja el logo con caracteres de bloque

#### Scenario: Terminal sin UTF-8
- **WHEN** la configuración regional no indica UTF-8
- **THEN** se dibuja una versión equivalente en ASCII

#### Scenario: Salida canalizada
- **WHEN** la salida no es un terminal
- **THEN** no se dibuja ningún logo

#### Scenario: Tamaño
- **WHEN** se dibuja el logo
- **THEN** ocupa como mucho tres líneas

### Requirement: La presentación no cuesta rendimiento

Agrupar la ayuda NO SHALL aumentar el número de procesos que lanza.

#### Scenario: Una sola consulta
- **WHEN** se ejecuta `hm --help`
- **THEN** el número de procesos `jq` que se lanzan no supera los que se lanzaban antes de agrupar

#### Scenario: Presupuesto respetado
- **WHEN** se mide `hm --help`
- **THEN** sigue dentro del presupuesto de rendimiento vigente
