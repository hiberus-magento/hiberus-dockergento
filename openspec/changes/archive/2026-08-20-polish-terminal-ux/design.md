## Context

Los colores se cargan hoy en `load_colors`, dentro de `load_properties.sh`, que se ejecuta
con `set -a`: las variables quedan exportadas y las heredan los comandos hijo. Toda la
salida pasa por `print_message.sh`, que ya interpola esas variables. Eso hace que apagar el
color sea un cambio de una sola pieza: si las variables están vacías, no hay color.

`print_message.sh` ya tiene además valores por defecto vacíos para todas ellas, que se
añadieron cuando los componentes empezaron a sourcearse en tests.

El flujo de preguntas vive en `console/components/input_info.sh`. `clear_screen` ya existe y
ya evita borrar cuando no hay TTY o el modo es no interactivo; el problema no es cuándo
borra, es que borre.

## Goals / Non-Goals

**Goals:**
- Que una contraseña no aparezca nunca en pantalla ni en el scrollback.
- Que la herramienta respete el estándar de color del ecosistema.
- Que preguntar no destruya el contexto que el usuario está leyendo.

**Non-Goals:**
- Rediseñar la presentación (eso es UX-04 y siguientes).
- Tocar el contrato de salida.

## Decisions

### 1. Un único punto de decisión para el color, con precedencia explícita

En `load_colors`, por orden de mayor a menor prioridad:

1. `--no-color` (o `HM_NO_COLOR`): sin color, siempre.
2. `NO_COLOR` definida y no vacía: sin color. Es el estándar de no-color.org.
3. `TERM=dumb` o `TERM` vacío: sin color.
4. `FORCE_COLOR` o `CLICOLOR_FORCE`: con color, **aunque la salida esté canalizada**. Existe
   para logs de CI que sí renderizan ANSI.
5. stdout no es un TTY: sin color.
6. En cualquier otro caso: con color.

El orden importa y no es arbitrario: la petición explícita del usuario gana al entorno, y el
forzado gana a la detección automática pero no a una petición explícita de *no* color.

Alternativa descartada: comprobar el color en cada función de `print_message.sh`. Sería
repetir la decisión veinte veces y dejar sitio donde olvidarla.

### 2. El modo JSON ya no es el único que apaga el color

Hoy `_print_decorated` decide entre stdout y stderr según el modo de salida, y en JSON
imprime sin color. Con este cambio, el color puede estar apagado por seis motivos distintos,
y la vía es siempre la misma: las variables vacías. `_print_decorated` no cambia.

### 3. La contraseña, sin eco y sin rastro

`read -rs` para la contraseña, más un salto de línea manual —porque sin eco el usuario no
deja el suyo al pulsar Enter y el siguiente mensaje se pegaría a la pregunta.

Se revisa además que la contraseña no acabe en ningún sitio inesperado: hoy `transfer-db`
la mete en una cadena `-p'…'` que se pasa a `mysql`, lo cual no la imprime, pero conviene
verificarlo y dejarlo anotado.

`hm describe --with-secrets` sigue mostrando credenciales: ahí es lo pedido explícitamente.

### 4. Nada de borrar pantalla en el flujo de preguntas

Se quitan las llamadas a `clear_screen` de `custom_question` y `custom_select`.

El caso incómodo es `version_manager`, que redibuja la tabla de requisitos cada vez que el
usuario edita una versión y hoy limpia antes. Sin borrado, las tablas se acumulan hacia
abajo. **Se acepta ese comportamiento**: ver la progresión de lo que has cambiado es más
informativo que perderla, y separar cada tabla con una línea en blanco basta para que se
lea. Un TUI es el sitio para redibujar en su lugar, y allí se hará con el buffer alternativo,
que se puede deshacer al salir.

`clear_screen` se mantiene como función, sin usos, marcada para que la use la biblioteca de
componentes (UX-07) cuando exista; o se elimina si allí no encaja.

### 5. mac y linux

Sin diferencias. `read -rs`, `NO_COLOR` y `TERM` se comportan igual en el Bash 3.2 de macOS
y en el 5.x de Linux.

## Risks / Trade-offs

- **Alguien depende del color en un pipeline** → para eso está `FORCE_COLOR`, y se documenta.
- **Las tablas de requisitos se acumulan** → aceptado y justificado en la decisión 4; si
  molesta en la práctica, el sitio de arreglarlo es el TUI.
- **Un `read -rs` mal puesto deja el terminal sin eco** → sólo se usa en una pregunta
  concreta y `read` restaura el estado al volver; se comprueba en la verificación manual.
- **Perder color en tests que lo esperaban** → las pruebas comparan contenido, no color, y
  ya hay valores por defecto vacíos para las variables de color.

## Migration Plan

Ninguna. Versión normal de `hm`. Conviene mencionar en el changelog que la herramienta ya
respeta `NO_COLOR`, porque hay gente que lo tiene puesto en su shell y notará el cambio.

## Open Questions

Ninguna.
