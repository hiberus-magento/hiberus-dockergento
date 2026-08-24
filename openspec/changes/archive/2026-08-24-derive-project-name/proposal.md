# El nombre del proyecto sale del directorio cuando nadie lo ha decidido

## Por qué

Hoy `hm setup` pregunta el nombre del proyecto y lo escribe en
`config/docker/properties.json`, que **está versionado**. Eso tiene dos consecuencias que
estorban:

**Dos copias del mismo repositorio son el mismo entorno.** Clonar un proyecto en otro
directorio para comparar una rama, o para que otra persona lo mire, produce un checkout con el
mismo `COMPOSE_PROJECT_NAME`. Los contenedores se pisan entre sí y los volúmenes se comparten
sin que nadie lo haya pedido.

**Hay un nombre vacío circulando por la herramienta.** Cuando la propiedad está vacía —un
proyecto que nunca pasó por `setup`, o alguien que la borró—, Docker Compose deriva el nombre del
directorio y sigue adelante, pero `hm` no: sigue creyendo que el nombre es la cadena vacía. Las
etiquetas `hm.*` se escriben vacías, la búsqueda de contenedores por servicio no encuentra nada y
`hm list` muestra el entorno por el nombre que puso Compose, no por el que la CLI cree tener.
Dos verdades distintas sobre la misma cosa.

DDEV ofrece precisamente esto como opción global, para que cada directorio sea un proyecto
distinto sin configurar nada.

## Qué cambia

- **Un nombre resuelto, uno solo.** Si hay nombre configurado, gana: para todos los proyectos que
  existen hoy no cambia absolutamente nada. Si no lo hay, se deriva del directorio *exactamente
  como lo haría Compose*, de modo que la CLI y Compose no puedan discrepar nunca.
- Ese nombre resuelto es el que usan las etiquetas, la búsqueda de contenedores, `describe`,
  `list` y el panel.
- **`hm setup` deja de fijar el nombre cuando coincide con el derivado.** Un proyecto nuevo no
  guarda nada, así que un clon en otro directorio se comporta como un entorno distinto sin tocar
  nada. Quien escriba un nombre propio lo sigue guardando.
- Un directorio del que no sale un nombre válido —`___`, o sólo símbolos— se rechaza con un
  mensaje que lo explica, en lugar del error de Compose.

## Qué no cambia

- **Ningún proyecto existente cambia de nombre.** Es la restricción que manda: cambiarlo dejaría
  huérfanos sus contenedores y, lo que importa de verdad, sus volúmenes.
- `COMPOSE_PROJECT_NAME` sigue siendo la propiedad que manda cuando está puesta, y se sigue
  pudiendo fijar a mano.
- Nada de esto habilita todavía un entorno por worktree: eso es WT-02, y los guardarraíles de
  worktree siguen en pie.

## Cómo se sabrá que funciona

- Un proyecto con nombre configurado se comporta igual que antes, bit a bit.
- Dos clones del mismo repositorio en directorios distintos, sin nombre configurado, son dos
  entornos independientes.
- El nombre que reporta `hm describe` es el mismo que reporta `docker compose config`, incluidos
  los directorios con mayúsculas, espacios, puntos o acentos.
- Las etiquetas `hm.project` dejan de escribirse vacías.
