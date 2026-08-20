## Context

Dockergento ya escribe secuencias ANSI de color a mano, sin `tput`, y eso está bien: `tput`
cuesta 10-15 ms por invocación y no aporta portabilidad real frente a VT100. Lo que falta es
todo lo demás: tamaño, cursor, pantalla y teclado.

La restricción que manda es **Bash 3.2**, la versión que trae macOS. Descarta arrays
asociativos, `read -N`, `${var@Q}` y `wait -n`.

La referencia práctica es la guía de escribir un TUI en Bash de la comunidad, cuyas
conclusiones coinciden con nuestras restricciones.

## Goals / Non-Goals

**Goals:**
- Una caja de herramientas pequeña, legible y sin dependencias, sobre la que se puedan
  construir la ayuda agrupada, el indicador de progreso, el selector y el TUI.
- Que el terminal quede **siempre** como estaba, incluso si el programa muere.
- Que todo sea inofensivo cuando no hay terminal.

**Non-Goals:**
- Un framework. Si hace falta un gestor de layouts, es que nos hemos equivocado de camino.
- Dibujar nada concreto.

## Decisions

### 1. `stty size` para el tamaño, con retrocesos

`stty size` es POSIX y funciona en cualquier versión de Bash; `checkwinsize` necesita Bash 4
y macOS trae 3.2. Orden: `stty size` → `$LINES`/`$COLUMNS` si están definidas → 24×80 como
último recurso, que es el tamaño que asume cualquier terminal desde 1978.

Alternativa descartada: preguntar la posición del cursor moviéndolo a la esquina. Funciona,
pero requiere leer la respuesta del terminal, con lo que hay que manejar tiempos de espera y
entrada en crudo para algo que `stty` resuelve en una línea.

### 2. Secuencias en crudo, y documentadas

`\e[?25l` y `\e[?25h` para ocultar y mostrar el cursor. `\e[<fila>;<col>H` para mover.
`\e[?1049h` y `\e[?1049l` para entrar y salir de la pantalla alternativa. `\e[<fila>;<fila>r`
para la región de scroll. `\e[2K` para borrar la línea.

Cada una se documenta en el propio fichero con lo que hace, porque dentro de un año nadie va
a recordar qué es `1049` y la tentación será sustituirlo por `clear`.

### 3. Pantalla alternativa en lugar de `clear`

Es la diferencia entre un TUI que respeta el terminal y uno que lo destroza: al salir de la
pantalla alternativa, el usuario recupera **exactamente** lo que tenía, incluido su
scrollback. `clear` lo borra sin vuelta atrás.

Esto es lo que permite que el TUI use pantalla completa sin caer en el defecto que estamos
arreglando en el flujo de preguntas.

### 4. Restaurar el terminal es responsabilidad del componente, no del que lo usa

Entrar en pantalla alternativa u ocultar el cursor deja el terminal en un estado que hay que
deshacer. Si el programa muere entre medias —un error, un `Ctrl-C`, un `kill`— el usuario se
queda con un terminal sin cursor y sin saber por qué.

Por eso, al entrar se instala un `trap` sobre `EXIT`, `INT` y `TERM` que restaura: pantalla
principal, cursor visible, eco activado, región de scroll completa. La restauración debe ser
**idempotente**, porque puede llegar por dos vías a la vez.

Es el requisito más importante de este cambio: un TUI que se cuelga y deja el terminal roto
quema la confianza para siempre.

### 5. Ceder el terminal y recuperarlo

Escribir la especificación del panel (TUI-01) sacó a la luz una operación que faltaba: el
panel no dibuja la salida de `hm start` dentro de un recuadro, **sale de la pantalla
alternativa**, deja que el comando escriba con normalidad y vuelve al terminar. Es el patrón
de `git` al abrir el editor, y evita tener que multiplexar salida.

Eso son dos funciones: ceder el terminal (salir de la pantalla alternativa, mostrar el
cursor, restaurar el eco) y recuperarlo (volver a entrar), ambas idempotentes y ambas
inofensivas sin terminal.

Y sacó a la luz lo contrario también: **la región inferior fija no tiene consumidor**. El
panel dibuja su línea de estado como parte de su propio contenido, y no hay nadie más que la
pida. Sale de este cambio, por el criterio que ya estaba escrito: nada entra por si acaso.

Ese es el motivo de haber especificado el consumidor antes de la biblioteca.

### 6. Sin terminal, todo es inofensivo

Cada función comprueba si stdout es un TTY y, si no lo es, no escribe nada y devuelve
correctamente. Así el mismo código sirve para una persona, para un pipe y para un agente,
que es la línea que ya sigue el resto de la CLI.

El tamaño es la excepción útil: sin terminal devuelve el valor por defecto, porque quien
formatea una tabla necesita un número con el que trabajar.

### 7. Separar calcular de emitir, para poder probarlo

Un componente que sólo escribe secuencias en un terminal es imposible de probar sin un
pseudo-terminal. Así que las funciones se parten en dos:

- **Calcular**: interpretar la salida de `stty size`, traducir una secuencia de teclas a un
  nombre (`arriba`, `abajo`, `enter`, `esc`), decidir la posición de un elemento. Son
  funciones puras y se prueban con entradas de mentira.
- **Emitir**: escribir la secuencia. Se prueba comprobando que no escribe nada cuando no hay
  terminal, y con un pseudo-terminal (`script`) para el camino interactivo.

Sin esta separación, la biblioteca queda sin pruebas, y es la base del TUI.

### 8. Lectura de teclas

`read -rsn1` para una pulsación. Las flechas llegan como tres bytes (`\e`, `[`, `A`), así que
al recibir `\e` se leen los siguientes con un tiempo de espera muy corto (`read -t 0.01`)
para distinguir una flecha de un `Esc` pulsado a solas. Bash 3.2 admite tiempos de espera
fraccionarios en `read -t`.

### 9. mac y linux

Sin diferencias en las secuencias. Sí en detalles: `stty size` existe en ambos con la misma
salida; `read -t` con decimales funciona en ambos. Se verifica en los dos.

## Risks / Trade-offs

- **Terminal roto tras un fallo** → decisión 4; es lo primero que se prueba.
- **Un terminal que no entienda alguna secuencia** → las elegidas son VT100/xterm, el mínimo
  común de cualquier terminal de los últimos treinta años; y sin TTY no se emite ninguna.
- **La biblioteca crece hasta ser un framework** → el criterio de admisión es que exista un
  consumidor concreto y ya especificado; nada entra "por si acaso".
- **Difícil de probar** → decisión 7.
- **`SIGWINCH` y `trap` interactuando mal** → el manejador de redimensionado sólo actualiza
  dos variables; no dibuja. Quien dibuja es el bucle principal del TUI.

## Migration Plan

No aplica: componente nuevo sin consumidores. Se puede revertir borrando el fichero.

## Open Questions

- ¿Debe el componente ofrecer una función de "dibujar tabla" o eso pertenece a cada
  consumidor? Propuesta: fuera de este cambio. Primero que existan dos consumidores y se vea
  qué comparten de verdad; con uno solo, el panel, no hay nada que compartir todavía.
