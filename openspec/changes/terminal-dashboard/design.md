## Context

Tres piezas ya construidas hacen que este panel sea presentación y poco más:

- `hm list --json` da la flota: nombre, estado, rama derivada, ruta, si es un worktree, si
  está huérfano y cuántos servicios corren de cuántos.
- `hm describe --json` da el detalle de un entorno: URLs, versión de Magento, estado por
  servicio, rutas, Xdebug.
- `hm doctor --json` da los problemas, con severidad y la acción que los arregla.

Y la biblioteca de componentes (UX-07) dará el tamaño del terminal, la pantalla alternativa,
el cursor, las teclas y `SIGWINCH`.

Los tiempos importan para el diseño: `list` tarda ~890 ms y `doctor` ~1,3 s. No son
instantáneos, así que el panel no puede quedarse en blanco mientras espera.

## Goals / Non-Goals

**Goals:**
- Responder de un vistazo a "qué tengo levantado y qué está mal".
- Operar lo frecuente sin escribir comandos ni cambiar de directorio.
- Devolver el terminal intacto al salir.

**Non-Goals:**
- Sustituir la CLI. El panel es un atajo, no la interfaz completa.
- Mostrar cosas que aún no existen.

## Decisions

### 1. Comando explícito, no secuestrar `hm` sin argumentos

DDEV abre su panel cuando se ejecuta sin argumentos, y es tentador copiarlo. Aquí no: `hm`
sin argumentos muestra la ayuda, que acabamos de agrupar precisamente para que sea el punto
de entrada de quien no sabe qué hacer. Cambiarlo sorprendería a todo el departamento y
tiraría ese trabajo.

Así que `hm tui`. Si con el tiempo el equipo lo usa a diario, cambiar el comportamiento por
defecto es una línea y una decisión aparte.

### 2. El panel presenta, la CLI decide

El panel **no consulta Docker**: pide los datos a `hm list`, `hm describe` y `hm doctor` en
JSON, y ejecuta acciones invocando `hm start`, `hm stop`, `hm restart`, `hm logs`.

Dos consecuencias que valen el precio de un proceso extra por acción:

- **Cero lógica duplicada.** Cuando `describe` aprenda a decir algo nuevo, el panel lo dirá
  sin tocarlo.
- **Las protecciones se heredan.** Los guardarraíles de worktree, los códigos de salida y el
  modo no interactivo ya están en esos comandos. Un panel que hablara con Docker
  directamente los saltaría todos.

### 3. Pintar antes de saber, y refrescar cuando lo pidas

Con `list` en ~890 ms, un panel que espere los datos antes de dibujar deja el terminal en
blanco casi un segundo. En su lugar: se dibuja el marco y una línea de estado que dice que
está cargando, y la lista aparece cuando llega.

El refresco es **manual**, con una tecla. No hay refresco automático por temporizador: cada
ciclo cuesta casi un segundo de Docker, y un panel abierto en una pestaña olvidada estaría
consultando la máquina para siempre. Si más adelante se quiere, será opt-in y con intervalo
explícito.

Alternativa descartada: cachear la salida entre aperturas. Un panel que muestra datos viejos
sin decirlo es peor que uno que tarda.

### 4. Dos vistas, no más

- **Flota**: una fila por entorno con nombre, estado, servicios corriendo, rama y ruta.
  Arriba, los avisos del diagnóstico global si hay alguno.
- **Detalle**: al abrir un entorno, lo que da `describe` —URLs, versión, servicios— y las
  acciones disponibles.

Nada de pestañas ni de paneles divididos. Si hacen falta tres vistas, es que el panel está
compitiendo con el dashboard web en lugar de complementarlo.

### 5. Teclas convencionales y descubribles

Flechas y `j`/`k` para moverse, `Enter` para abrir, `Esc` para volver, `q` para salir. Las
acciones con su inicial en inglés: `s` arrancar (*start*), `x` parar (*stop*), `r` reiniciar,
`l` logs, `o` abrir en el navegador, `d` diagnóstico, `?` ayuda de teclas.

La última línea muestra siempre las teclas disponibles en el contexto actual, porque un panel
cuyas teclas hay que memorizar de la documentación no se usa.

`Ctrl-C` sale, como en cualquier sitio.

### 6. Las acciones largas ceden el terminal

`hm start` tarda segundos y `hm logs -f` no termina. Intentar meter su salida dentro del
panel obligaría a multiplexar; en su lugar, el panel **sale de la pantalla alternativa**,
ejecuta el comando de forma normal —con su salida tal cual— y vuelve al panel al terminar.

Es el patrón que usa `git` para abrir el editor, y tiene una ventaja: si el comando falla,
el usuario ve el error completo, no una versión recortada dentro de un recuadro.

### 7. Sin terminal no hay panel

Ejecutar `hm tui` con la salida canalizada o en un script no dibuja nada: falla con el código
de argumentos inválidos y explica que necesita un terminal, y sugiere `hm list --json` para
lo que probablemente se quería. Un panel que escribe secuencias de control en un fichero de
log no ayuda a nadie.

### 8. Lo que aún no existe tiene su sitio reservado, vacío

Snapshots (DB-01) y entornos por worktree (WT-02) son las dos cosas que más pide este panel,
y ninguna existe. No se dibujan huecos "en construcción": cuando existan, se añaden. El único
guiño es que la vista de flota ya distingue los worktrees, porque `hm list` ya lo informa.

### 9. mac y linux

Sin diferencias, salvo lo que ya resuelve la biblioteca de componentes. `hm logs` y
`hm start` se comportan igual en ambas.

## Risks / Trade-offs

- **El terminal queda roto si el panel muere** → lo cubre la biblioteca (UX-07), y es su
  requisito más importante; aquí se comprueba de punta a punta.
- **Un proceso por acción es más lento que hablar con Docker** → aceptado a cambio de no
  duplicar lógica ni saltarse las protecciones (decisión 2).
- **El panel se convierte en un gestor de Docker** → el criterio de la decisión 4 y del
  documento de investigación: si no habla de proyectos, entornos o worktrees, no entra.
- **Datos obsoletos en pantalla** → la línea de estado muestra cuándo se cargaron, y el
  refresco es explícito.
- **Terminales muy estrechos** → la vista de flota recorta la ruta antes que el nombre y el
  estado, que son lo que se busca.

## Migration Plan

No aplica: comando nuevo, sin efectos sobre proyectos.

## Open Questions

- ¿Debe el panel poder cambiar de proyecto el directorio de trabajo del shell al salir? Es la
  petición que aparece sola en cuanto lo usas ("ábreme una terminal ahí"), y no se puede
  hacer desde un proceso hijo sin cooperación del shell. Propuesta: fuera de este cambio;
  si se quiere, se resuelve con una función de shell opcional, como hacen otras herramientas.
