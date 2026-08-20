# terminal-control Specification

## Purpose
TBD - created by archiving change terminal-components. Update Purpose after archive.
## Requirements
### Requirement: Tamaño del terminal

El componente SHALL informar del número de filas y columnas del terminal, y SHALL devolver
un valor utilizable cuando no haya terminal.

#### Scenario: Terminal disponible
- **WHEN** se pide el tamaño con un terminal conectado
- **THEN** se obtienen las filas y las columnas reales

#### Scenario: Sin terminal
- **WHEN** se pide el tamaño y la salida no es un terminal
- **THEN** se obtiene un tamaño por defecto de 24 filas y 80 columnas

#### Scenario: Ventana redimensionada
- **WHEN** la ventana cambia de tamaño y se vuelve a pedir
- **THEN** se obtiene el tamaño nuevo

### Requirement: Control del cursor y de la pantalla

El componente SHALL permitir ocultar y mostrar el cursor, moverlo, y entrar y salir de una
pantalla alternativa.

#### Scenario: Pantalla alternativa
- **WHEN** se entra en la pantalla alternativa, se escribe algo y se sale
- **THEN** el terminal muestra exactamente el contenido que tenía antes de entrar
- **AND** el scrollback anterior sigue disponible

#### Scenario: Cursor oculto y recuperado
- **WHEN** se oculta el cursor y después se muestra
- **THEN** el cursor vuelve a ser visible

#### Scenario: Ceder el terminal a otro comando
- **WHEN** se cede el terminal, otro comando escribe su salida y después se recupera
- **THEN** la salida de ese comando se ve con normalidad
- **AND** al recuperarlo se vuelve a la pantalla alternativa

#### Scenario: Ceder y recuperar repetidamente
- **WHEN** se cede y se recupera el terminal varias veces
- **THEN** el estado del terminal es correcto en cada paso

#### Scenario: Nada que emitir sin terminal
- **WHEN** cualquiera de esas operaciones se ejecuta con la salida no conectada a un terminal
- **THEN** no se escribe ninguna secuencia de control
- **AND** la función termina correctamente

### Requirement: El terminal queda siempre restaurado

El componente SHALL devolver el terminal a su estado original al terminar, incluso cuando el
programa acabe por una interrupción o por un error.

#### Scenario: Salida normal
- **WHEN** un programa que entró en la pantalla alternativa termina con éxito
- **THEN** el terminal queda en la pantalla principal, con el cursor visible y el eco activado

#### Scenario: Interrupción del usuario
- **WHEN** el usuario interrumpe con Ctrl-C mientras el programa dibuja
- **THEN** el terminal queda igualmente restaurado

#### Scenario: Error inesperado
- **WHEN** el programa muere por un error mientras dibuja
- **THEN** el terminal queda igualmente restaurado

#### Scenario: Restauración repetida
- **WHEN** la restauración se ejecuta dos veces
- **THEN** el terminal queda correctamente restaurado y no se produce ningún error

### Requirement: Lectura de teclas

El componente SHALL leer una pulsación de tecla y SHALL distinguir las flechas, Enter y Esc.

#### Scenario: Tecla normal
- **WHEN** el usuario pulsa una tecla imprimible
- **THEN** se obtiene ese carácter

#### Scenario: Flechas
- **WHEN** el usuario pulsa una flecha
- **THEN** se obtiene el nombre de la dirección y no la secuencia de escape en crudo

#### Scenario: Esc a solas
- **WHEN** el usuario pulsa Esc sin que le siga una secuencia
- **THEN** se obtiene Esc y no se queda esperando indefinidamente

#### Scenario: Enter
- **WHEN** el usuario pulsa Enter
- **THEN** se obtiene el nombre correspondiente

### Requirement: Interpretación separada de la emisión

Las funciones que interpretan datos SHALL poder probarse sin un terminal.

#### Scenario: Interpretación del tamaño
- **WHEN** se interpreta una salida de `stty size` dada como texto
- **THEN** se obtienen las filas y las columnas sin necesidad de un terminal

#### Scenario: Interpretación de una secuencia de teclas
- **WHEN** se interpreta una secuencia de teclas dada como texto
- **THEN** se obtiene el nombre de la tecla sin necesidad de un terminal

#### Scenario: Compatibilidad con Bash 3.2
- **WHEN** el componente se carga en el Bash 3.2 que trae macOS
- **THEN** se carga sin errores y todas sus funciones responden

