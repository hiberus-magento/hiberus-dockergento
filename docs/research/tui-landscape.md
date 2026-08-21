# Cómo se hace un TUI en Bash, y quién lo ha hecho antes

**Fecha**: 21 de agosto de 2026
**Motivo**: `hm tui` salió correcto pero tosco —parpadeaba y respondía tarde—, y antes de
seguir añadiéndole cosas convenía saber si el problema era Bash o era nuestro.
**Conclusión**: era nuestro. Medido y corregido en
[smooth-dashboard-rendering](../../openspec/changes/archive/).

---

## 1. La medida, primero

Con diez entornos, en un portátil con bash 3.2:

| | Antes | Después |
|---|---|---|
| Fotograma completo | **404 ms** | **1,7 ms** |
| └ filas de la flota | 179 ms (1 `jq` + ~30 subshells) | 0 (compuestas al cargar) |
| └ cabecera y contador | ~198 ms (dos `jq`, uno de ellos dos veces) | 0 |
| └ avisos del doctor | 27 ms | 0 |
| Componer al cargar | — | 85 ms, una vez por carga o redimensión |

La causa del parpadeo era independiente y más simple: se borraba la pantalla entera con
`\e[2J` antes de redibujar, así que el terminal mostraba una pantalla vacía durante un
refresco.

**La lección general**: en un TUI en Bash, el coste de un fotograma es el número de procesos
que lanza. `jq` cuesta ~36 ms, cada subshell unos 3 ms, y un `awk` para partir cinco números
cuesta lo mismo que calcularlos. Un fotograma que no lanza procesos es texto en variables, y eso
Bash lo hace rápido.

## 2. TUIs en Bash puro: qué han resuelto ya

### bashtop — `github.com/aristocratos/bashtop`

El TUI en Bash más ambicioso que existe: un monitor de recursos con gráficas, gradientes de 24
bits y refresco continuo. Dos cosas que aprender de él:

- **La técnica**: búferes por widget y **una sola escritura por fotograma**. Sólo se re-emite lo
  que ha cambiado. Es exactamente lo que nos faltaba, y lo que hemos adoptado en su forma
  simple: el fotograma se ensambla en una variable y se emite con un `printf`.
- **El techo**: su autor acabó reescribiéndolo en Python (bpytop) y después en C++
  (btop). Conviene saber dónde está ese techo — lo alcanzó con gráficas y refresco continuo,
  no con una tabla que se redibuja al pulsar una tecla. Nosotros estamos muy por debajo.

### fff — `github.com/dylanaraps/fff`

Gestor de ficheros en Bash puro, sin dependencias. Es el caso más parecido al nuestro: lista
navegable, teclas, sin librerías. Referencia de cómo dibujar sólo con `printf` y secuencias
crudas, y de cuánto se puede hacer sin salir del intérprete. Archivado, pero el código sigue
siendo la mejor lectura del género.

### pure-bash-bible — `github.com/dylanaraps/pure-bash-bible`

No es un TUI: es el recetario de "esto sin lanzar un proceso". Directamente aplicable, porque
nuestro coste **eran** los procesos: `${var:0:n}` en vez de `cut`, `${#var}` en vez de `wc -c`,
`printf -v` en vez de `var=$(printf ...)`, expansión de parámetros en vez de `sed`.

## 3. Widgets de verdad llamables desde Bash

Si algún día queremos más que una tabla, estas son las salidas sin reescribir la herramienta:

| Herramienta | Qué es | Coste | Cuándo tendría sentido |
|---|---|---|---|
| **fzf** — `github.com/junegunn/fzf` | Buscador difuso con panel de vista previa, atajos configurables (`--bind`) y recarga en vivo (`reload`) | Un binario, ya instalado en muchas máquinas | Selección entre muchos entornos, o un panel con vista previa sin escribirlo nosotros |
| **gum** — `github.com/charmbracelet/gum` | Widgets sueltos (`choose`, `filter`, `input`, `table`, `spin`) para scripts de shell | Un binario Go | Formularios y asistentes (`hm setup`, `hm install`) con aspecto moderno |
| **dialog / whiptail** | Widgets clásicos sobre ncurses; lo que usan `raspi-config` y `debconf` | Dependencia del sistema | Menús en máquinas donde ya están; el aspecto es de otra época |

**Decisión vigente**: ninguno. Un único camino, sin aceleradores opcionales, para no tener dos
comportamientos que documentar y mantener según lo que haya instalado. Si se revisa, `fzf` es el
primer candidato por lo extendido que está.

## 4. La vara de medir: TUIs que no son Bash

Ninguno de estos está escrito en shell, y por eso mismo marcan el listón de lo que la gente
espera de un panel de terminal:

- **lazydocker** (`github.com/jesseduffield/lazydocker`) — el competidor funcional directo de
  nuestro panel: contenedores, logs, estadísticas.
- **ctop** (`github.com/bcicen/ctop`) y **dry** (`github.com/moncho/dry`) — la misma idea, más
  ligeros.
- **k9s** (`github.com/derailed/k9s`) — para Kubernetes; el referente de UX del género.
- **btop** (`github.com/aristocratos/btop`) — el final de la historia de bashtop.

Lo que tienen y nosotros no: navegación por paneles, logs en vivo dentro de la interfaz,
filtrado incremental. Lo que nosotros tenemos y ellos no: saben de contenedores, no de
Dockergento. Ninguno sabe qué es un worktree de un proyecto Magento.

Y el hueco sigue abierto: **ni DDEV ni Warden tienen TUI**.

## 5. Utilidades de alrededor

- **vhs** (`github.com/charmbracelet/vhs`) — guioniza una sesión de terminal en un `.tape` y
  produce un GIF. Sirve para el README y para revisar de un vistazo un cambio de render.
- **El pseudo-terminal como test**: nuestras pruebas del panel usan `script(1)` con las teclas
  inyectadas desde un fichero. Para lo que `script` no permite —cambiar el tamaño de la ventana
  a mitad de sesión— hace falta un pty de verdad; con el módulo `pty` de Python son quince
  líneas, y así se verificó que redimensionar repinta en el acto.

## 6. Las seis reglas que sacamos de todo esto

Aplicadas ya en `console/tasks/tui_frame.sh`:

1. **Componer cuando llegan los datos, no cuando se pulsa una tecla.** El ancho del terminal es
   una entrada de la composición, no del pintado.
2. **Nunca `\e[2J` entre fotogramas.** Cursor al origen, sobrescribir cada línea con `\e[K`, y
   limpiar hacia abajo al final.
3. **Un fotograma, una escritura.** Cada `printf` es una llamada al sistema y una oportunidad de
   ver la pantalla a medio dibujar.
4. **Salida sincronizada** (`\e[?2026h`/`\e[?2026l`): el terminal presenta el fotograma completo.
   Quien no conoce el modo lo ignora.
5. **Rellenar el fotograma hasta el borde inferior.** Si se reescribe cada celda, no puede
   quedar nada del fotograma anterior.
6. **Cero procesos en el camino de pintado.** Verificado por un test que prohíbe `jq` y `awk`
   después de componer, y por un presupuesto de 20 ms por fotograma.
