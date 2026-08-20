# Investigación: interfaz de terminal y experiencia de uso

> Estado: **investigación / propuesta**. Nada implementado.
> Relacionado: [control-plane-ui.md](control-plane-ui.md) (el TUI y el dashboard),
> [ai-features.md](ai-features.md) (el modo no interactivo, ya implementado).

---

## 1. Por qué ahora

El TUI (TUI-01 del backlog) va a ser la primera interfaz *visual* de Dockergento, y no se
puede construir sobre una capa de presentación que hoy no sabe ni cuánto mide el terminal.
Antes de dibujar una flota conviene arreglar los cimientos: colores, preguntas, anchura y
señales de progreso.

Además hay defectos concretos que ya afectan al uso diario, y uno de ellos es de seguridad.

## 2. Estado actual, auditado

Lo que **sí** tenemos, y está bien: 18 funciones de salida semántica en
`print_message.sh` (`print_info`, `print_error`, `print_code`, `print_link`…), separación
de stdout/stderr ya resuelta, modo JSON, modo no interactivo y **cero dependencias**. No se
usa `tput` en ningún sitio, lo cual es correcto: cada invocación cuesta 10-15 ms.

Lo que **no** tenemos:

| Hueco | Consecuencia |
|---|---|
| No se honra `NO_COLOR` ni `TERM=dumb` ni existe `--no-color` | La herramienta ignora el estándar que respeta el resto del ecosistema |
| No se conoce la anchura del terminal | La ayuda y las tablas se rompen en ventanas estrechas; el sangrado es con tabuladores |
| **Las contraseñas se escriben en claro** | `hm transfer-db` pide la contraseña de la base de datos con `read -p`, sin `-s`: se ve en pantalla y queda en el scrollback |
| **Cada pregunta borra la pantalla** | `custom_question` y `custom_select` llaman a `clear`: en `hm setup` cada respuesta borra lo que acabas de leer |
| El selector es el `select` de Bash | Lista numerada, escribir un número, sin valor por defecto, sin flechas, y "Invalid option" como único mensaje de error |
| No hay señal en operaciones largas | `composer install` o `setup:upgrade` pueden estar minutos sin decir nada |
| La ayuda es una lista plana de 45 comandos | Sin agrupar, sin ejemplos, sin línea de uso, sin "lo más común primero" |

## 3. La referencia: clig.dev

Las *Command Line Interface Guidelines* son el documento de consenso sobre esto. Las reglas
que nos afectan, y si las cumplimos:

| Regla | Estado |
|---|---|
| "Solo usa prompts si stdin es un terminal interactivo" | ✅ resuelto con el modo no interactivo |
| "Nunca *exijas* un prompt": siempre debe haber flag equivalente | ⚠️ parcial: `transfer-db` no tiene flags para todo lo que pregunta |
| "Si pides una contraseña, no la imprimas mientras se escribe" | ❌ **incumplido** |
| "Desactiva el color si no hay TTY, si `NO_COLOR` está definida, si `TERM=dumb` o si el usuario pasa `--no-color`" | ⚠️ solo el caso de "no hay TTY" |
| "Usa el color con intención", no como decoración | ⚠️ hoy casi toda la salida va coloreada |
| "Imprime algo en menos de 100 ms" y "muestra progreso si algo tarda" | ❌ |
| "Si stdout no es un terminal, nada de animaciones" | — (no hay animaciones) |
| "Empieza por ejemplos" en la ayuda | ❌ |
| "Muestra primero los comandos y flags más comunes" | ❌ lista alfabética plana |
| "Confirma antes de algo peligroso"; para lo severo, pide escribir el nombre del recurso | ⚠️ `down -v` no confirma |
| "Deja escapar al usuario": `Ctrl-C` siempre funciona | ✅ |
| "Reescribe los errores para humanos", con la acción a tomar | ✅ resuelto en el contrato de salida y en `doctor` |

## 4. Cómo lo hace la comunidad

### 4.1 Estándares de color

Tres variables de entorno se han consolidado, y son triviales de implementar:

- **`NO_COLOR`** (no-color.org): si está definida y no está vacía, sin color. La respetan
  Docker, git, ripgrep, fd, bat y prácticamente todo lo moderno.
- **`FORCE_COLOR`** / **`CLICOLOR_FORCE`**: color aunque la salida esté canalizada, para
  logs de CI que sí renderizan ANSI.
- **`TERM=dumb`**: terminal sin capacidades; ni color ni secuencias de control.

### 4.2 TUI en Bash puro

La referencia práctica es `dylanaraps/writing-a-tui-in-bash`, y sus conclusiones encajan
con nuestras restricciones:

- **Secuencias VT100 en crudo, no `tput`**: menos sobrecarga y ninguna dependencia de
  ncurses. Ya lo hacemos.
- **Tamaño del terminal con `stty size`**: es POSIX y funciona en cualquier versión de
  Bash, frente a `checkwinsize` que necesita Bash 4 (y macOS trae 3.2).
- **Buffer de pantalla alternativo**: `\e[?1049h` para entrar y `\e[?1049l` para salir. Es
  la forma correcta de que un TUI no destruya el scrollback del usuario — exactamente lo
  contrario de lo que hace hoy nuestro `clear`.
- **Ocultar el cursor** (`\e[?25l` / `\e[?25h`), mover el cursor (`\e[<fila>;<col>H`),
  región de scroll (`\e[0;10r`) para dejar una línea fija abajo.
- **`SIGWINCH`** para redibujar al cambiar el tamaño de la ventana.
- Todo esto funciona en **Bash 3.2**, que es lo que hay en macOS.

### 4.3 Dependencias opcionales, nunca obligatorias

El patrón que usan las herramientas cuidadas: si `fzf` o `gum` están instalados, se
aprovechan; si no, hay un camino nativo. Ya lo decidimos así para el TUI y aplica igual al
selector de opciones.

### 4.4 Ayuda agrupada

`docker`, `gh` y `ddev` agrupan los comandos por propósito en lugar de listarlos
alfabéticamente. Nosotros ya tenemos la clasificación hecha —informativos, de flujo de
datos, de paso, de control— en el inventario del contrato de salida; solo hay que usarla
para presentar.

## 5. Propuestas

Ordenadas por relación entre lo que arreglan y lo que cuestan.

### 5.1 Contraseñas sin eco *(pequeño, y es seguridad)*

`read -s` en la pregunta de contraseña de `transfer-db`, con un salto de línea manual
después. Hoy la contraseña de la base de datos de un cliente queda en pantalla y en el
scrollback.

### 5.2 Honrar los estándares de color *(pequeño)*

Un único punto de decisión —`NO_COLOR`, `TERM=dumb`, `--no-color`, `FORCE_COLOR`, TTY— que
resuelva si `load_colors` define secuencias o cadenas vacías. Como los colores ya salen de
variables, el resto del código no se toca.

### 5.3 Dejar de borrar la pantalla al preguntar *(pequeño)*

Quitar el `clear` de `custom_question` y `custom_select`. Que una pregunta borre el contexto
que el usuario acaba de leer es de las cosas que más desorientan, y en `setup` pasa en cada
respuesta. El sitio donde el borrado sí tiene sentido es un TUI, y ahí se hace entrando al
buffer alternativo, que se puede deshacer al salir.

### 5.4 Ayuda agrupada, con uso y ejemplos *(medio)*

```
Usage: hm <command> [options]

Environment       start · stop · restart · rebuild · down · setup · doctor
Magento           magento · composer · install · purge · test-unit · test-integration
Database          mysql · mysqldump · transfer-db · masquerade
Inspection        describe · list · logs · compatibility
...

Examples
  hm start                     Bring the environment up
  hm magento cache:clean       Run a Magento command
  hm mysql -i dump.sql         Import a database dump

Run 'hm <command> --help' for details.
```

Los grupos salen de una clave nueva en `command_descriptions.json`, así que la
presentación no queda cableada en el código. Y con la anchura del terminal (`stty size`) se
decide si caben en columnas o en lista.

### 5.5 Señal en operaciones largas *(medio)*

Una función `with_progress <mensaje> <orden…>` que muestre un indicador mientras la orden
corre y lo borre al terminar, **solo si stdout es un TTY** y no estamos en modo JSON. Lo
importante no es el spinner: es la regla de clig.dev de **imprimir algo antes de 100 ms**,
que hoy incumplimos en `composer install` y en `setup:upgrade`.

### 5.6 Selector navegable *(medio)*

Un `custom_select` que use flechas y `Enter`, con la opción por defecto preseleccionada,
leyendo teclas de una en una (`read -rsn1`). Con retroceso a la lista numerada actual si el
terminal no lo soporta, y a `fzf` si está instalado.

### 5.7 Biblioteca de componentes de terminal *(medio, habilita el TUI)*

`console/components/tui.sh` con lo mínimo del §4.2: tamaño, cursor, buffer alternativo,
`SIGWINCH`, lectura de teclas, línea fija inferior. Es la base sobre la que se dibuja el
TUI, y conviene que exista antes de empezarlo.

## 6. Qué no haríamos

- **Ninguna dependencia obligatoria**: ni `gum`, ni `dialog`, ni `whiptail`, ni ncurses.
  Como opcionales, bienvenidas.
- **Ni `tput`**: 10-15 ms por invocación y no aporta portabilidad frente a VT100.
- **Colores como decoración**: el color señala estado (error, aviso, correcto) o resalta lo
  accionable. Lo demás, en el color del terminal.
- **Animaciones cuando no hay TTY**: convierten los logs de CI en basura.
- **Emoji como única señal**: los emoji acompañan, pero el estado se dice con palabras
  (`OK` / `WARN` / `FAIL`), que es lo que funciona en un lector de pantalla y en un log.

## 7. Cómo encaja con el TUI

El orden importa: **primero los cimientos, después el dibujo**. El TUI necesita saber la
anchura, entrar y salir del buffer alternativo sin destruir el scrollback, leer teclas y
redibujar en `SIGWINCH`. Todo eso es §5.7, y de paso arregla la experiencia de los comandos
normales, que es lo que el equipo usa cien veces al día.

Y hay una simetría útil con lo que ya está hecho: el contrato de salida separó *qué* se
dice de *cómo* se dice. Esta línea de trabajo es la mitad "cómo".
