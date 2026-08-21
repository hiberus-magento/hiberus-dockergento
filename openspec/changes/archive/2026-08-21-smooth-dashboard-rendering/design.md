## Context

`hm tui` dibuja así: borrar la pantalla, recalcular todo desde JSON, escribir el resultado
con decenas de `printf`. Es la forma más directa de escribir un TUI y también la que produce
exactamente los dos defectos que se ven: parpadeo y latencia.

Medido en esta máquina con diez entornos: **404 ms por fotograma**, de los cuales 179 ms son
`tui_fleet_rows` (un `jq` más ~30 subshells de recorte, tres por fila) y ~198 ms dos llamadas
a `jq` que se repiten dentro del mismo fotograma (`tui_fleet_header`, `tui_fleet_count`, ésta
además invocada dos veces: en el bucle principal y dentro de `draw_fleet`).

El dato relevante es que **nada de eso depende de la tecla pulsada**. Los datos cambian cuando
se cargan (al abrir, al refrescar con `g`, al terminar una acción); entre esos momentos, mover
la selección es un cambio de dos líneas sobre un contenido idéntico.

En el otro extremo hay un techo real: bashtop, el TUI en Bash más ambicioso que existe,
terminó reescrito en Python y luego en C++. Pero llegó a ese techo con gráficas, gradientes de
24 bits y refresco continuo; una tabla de veinte filas que se redibuja al pulsar una tecla está
muy por debajo. La técnica que usaba bashtop antes de rendirse —búferes por widget y una sola
escritura por fotograma— es precisamente la que aquí falta.

## Goals / Non-Goals

**Goals**
- Un fotograma por debajo de 20 ms con veinte entornos.
- Cero parpadeo: el terminal nunca muestra una pantalla vacía ni un fotograma a medias.
- Mover la selección no lanza procesos.
- Sin dependencias nuevas y sin cambiar el comportamiento visible.

**Non-Goals**
- No se añaden vistas, teclas ni funcionalidad.
- No se introduce `fzf`, `gum`, `dialog` ni ninguna librería: la decisión de una única
  experiencia sin aceleradores opcionales sigue en pie.
- No se persigue refresco continuo ni animación. El panel se refresca cuando se le pide.
- No se reescribe el TUI en otro lenguaje. Si algún día hace falta, el contrato JSON ya
  permite hacerlo sin tirar nada.

## Decisions

### 1. El modelo: datos → líneas → fotograma

Tres etapas con fronteras claras, en lugar de una función que hace las tres cosas a la vez:

1. **Cargar** (`load_fleet`, `load_detail`): habla con la CLI, produce JSON. Cuesta lo que
   cuesta —son procesos— y sólo ocurre cuando el usuario lo provoca.
2. **Componer** (nuevo): del JSON a un array de líneas ya recortadas al ancho. Es donde vive
   `jq` y los recortes. Se ejecuta al cargar y al cambiar el tamaño del terminal.
3. **Pintar**: del array de líneas a bytes en el terminal. Sin procesos, sin `jq`, sin
   subshells: sólo expansión de variables.

La consecuencia de diseño importante: **el ancho del terminal es una entrada de la etapa 2, no
de la 3**. Redibujar tras un `SIGWINCH` recompone; redibujar por una tecla no.

### 2. Un array de líneas, no una cadena

Bash 3.2 no tiene arrays asociativos, pero sí indexados, que es lo que hace falta. Guardar el
fotograma como `FRAME[0..n]` permite lo que una sola cadena no permite: repintar la línea 7 sin
tocar las demás. El repintado parcial es el que hace que mover la selección se sienta
instantáneo, y no se puede hacer sobre una cadena única.

### 3. Nunca borrar la pantalla entre fotogramas

`\e[2J` es la causa del flash: entre el borrado y la escritura hay al menos un refresco del
terminal en el que no hay nada. El patrón correcto:

```
\e[H                      cursor al origen (no borra nada)
línea + \e[K              escribir y borrar lo que quedara a la derecha
...
\e[J                      al terminar, borrar de aquí hacia abajo
```

Cada celda se sobrescribe con su nuevo contenido en el mismo paso en que se borra el antiguo,
así que no hay ningún instante con la pantalla vacía. `\e[2J` se sigue usando una vez, al
entrar en la pantalla alternativa, donde sí hay que partir de un lienzo limpio.

### 4. Un fotograma es una escritura

Cada `printf` es una llamada al sistema, y el terminal pinta lo que le va llegando: cuarenta
escrituras son cuarenta oportunidades de ver la pantalla a medio dibujar. El fotograma se
ensambla completo en una variable y se emite con un único `printf '%s'`.

Esto obliga a eliminar los `| while IFS= read -r` del camino de pintado, que además de
forkear escribían línea a línea. Bash 3.2 puede recorrer un array indexado sin pipe.

### 5. Synchronized output

`\e[?2026h` … `\e[?2026l` alrededor del fotograma le dice al terminal que no muestre nada hasta
que termine: el equivalente a un vsync. Los terminales que no conocen el modo lo ignoran, que
es cómo funcionan los modos privados DEC, así que no hace falta detectar nada ni mantener dos
caminos. Es una línea de código y el resultado es visible incluso con el resto ya arreglado.

### 6. Sin repintado parcial: la medida lo desaconseja

El plan era repintar sólo las dos filas que cambian al mover la selección. Con las etapas
separadas, un fotograma completo mide **1,7 ms** (100 fotogramas, veinte entornos, 120
columnas). El repintado parcial ahorraría alrededor de un milisegundo de un presupuesto de
dieciséis por refresco de pantalla: imperceptible.

Se descarta, y lo que se gana es no tener dos caminos de pintado. Un fotograma completo es
siempre correcto por construcción —cada celda se reescribe—, mientras que el parcial depende de
que el resto de la pantalla siga siendo lo que se cree que es. Cambiar código correcto por
código con una suposición, para ahorrar un milisegundo, es un mal cambio.

### 7. Sin descarte de teclas encoladas: bash 3.2 no lo permite

El plan era consumir las pulsaciones pendientes antes de pintar. Requiere preguntar si hay
entrada disponible sin bloquear, y en bash 3.2 no hay forma de hacerlo: `read -t 0` se
introdujo en bash 4 y en la versión de macOS devuelve 1 aunque haya datos esperando —
comprobado—, `read -t` fraccionario está rechazado, y las alternativas (`stty` no bloqueante
alternando en cada tecla, o un sondeo con `sleep`) cuestan un proceso por pulsación o gastan CPU
en reposo.

Se descarta, y con el fotograma en 1,7 ms deja de importar: las pulsaciones de una tecla
mantenida llegan cada ~33 ms, veinte veces más lentas de lo que cuesta atenderlas. El encolado
era un síntoma de los 400 ms, no un problema por sí mismo.

### 8. Redimensionar repinta en el momento

Al separar las etapas apareció un defecto que estaba tapado: la tecla se leía con
`key=$(tui_read_key)`, es decir **dentro de un subshell**. La señal de redimensionado que llega
mientras se espera una tecla la atiende ese subshell, así que el nuevo tamaño se actualizaba
donde nadie podía verlo, y el panel seguía con el ancho anterior hasta la siguiente tecla.

La lectura pasa a asignar a una variable en lugar de escribir —el mismo patrón que el resolutor
de worktrees—, con lo que el manejador de la señal corre en la shell principal y puede
recomponer y repintar en el acto. Verificado con un pty de verdad: tras cambiar el tamaño y
enviar la señal, sin pulsar ninguna tecla, aparece un fotograma nuevo con las filas hasta la 38
y el pie en su forma larga.

Como efecto secundario, `read` no se interrumpe: bash reanuda la lectura después de ejecutar el
manejador, comprobado por separado. El panel no se cierra al redimensionar.

### 9. El pie se elige por lo que cabe

El pie tenía dos formas y un umbral de columnas elegido a ojo: la larga a partir de 100. La
larga mide 110 caracteres, así que a 100 columnas se elegía y se recortaba, y lo que se perdía
por la derecha era `q quit`. Ahora se mide la cadena y se usa la que entra.

### 8. Presentación comprobable sin terminal

La etapa 2 sigue siendo funciones puras de JSON a texto, que es lo que permite probar el
layout sin pty. La etapa 3 se prueba por lo que **no** emite: sin `\e[2J`, con las marcas de
synchronized output, y en una sola escritura. Esto último se comprueba contando escrituras con
`strace`/`dtruss`... que requiere privilegios en mac, así que en su lugar se comprueba lo
observable: que el fotograma es una única cadena en una variable antes de emitirse.

## Risks / Trade-offs

- **Recomponer al redimensionar puede notarse.** Cambiar el tamaño de la ventana dispara la
  etapa 2, que sí cuesta. Es aceptable: redimensionar es un gesto raro y deliberado, y
  encadenar redimensiones sólo recompone una vez porque el `SIGWINCH` se colapsa.
- **Un fotograma completo por tecla escribe unos 2 KB.** Es una escritura y el terminal la
  absorbe sin esfuerzo; a cambio no hay ningún estado de pantalla que mantener sincronizado.
- **Una flota más alta que el terminal necesita ventana.** Con posicionamiento absoluto, lo que
  cae por debajo del borde simplemente no se ve, así que la lista se desplaza para mantener la
  selección visible y una línea dice qué parte se está mostrando. No hacerlo habría convertido
  una lista larga en una selección invisible.
- **`\e[?2026` en terminales antiguos**: ignorado. El riesgo real sería un terminal que
  respondiera algo por el canal de entrada, y los modos DEC privados no responden.

## Migration Plan

No hay migración: el comando es interno y salió en la 1.5.0 sin publicarse todavía en una
release final. Nadie depende de su forma de pintar.

## Open Questions

Ninguna abierta. Si tras esto sigue sintiéndose corto para logs en vivo o gráficas, esa es una
conversación distinta —aceleradores opcionales o otro lenguaje— y no la resuelve este cambio.
