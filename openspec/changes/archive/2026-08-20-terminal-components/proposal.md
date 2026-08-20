## Why

El TUI (TUI-01) va a ser la primera interfaz visual de Dockergento, y hoy la capa de
presentación **no sabe ni cuánto mide el terminal**. Tampoco sabe entrar y salir de una
pantalla propia sin destruir el scrollback del usuario, ni leer una tecla, ni redibujar
cuando la ventana cambia de tamaño.

Sin esa base, el TUI se construiría a base de `clear` y de suposiciones sobre el ancho, que
es exactamente lo que hace hoy el flujo de preguntas y lo que estamos corrigiendo.

Y no es trabajo que sólo sirva al TUI: la anchura del terminal hace falta ya para la ayuda
agrupada (UX-04), y el indicador de operaciones largas (UX-05) necesita cursor y línea fija.

Backlog: **UX-07**. Es prerrequisito de **TUI-01**, **UX-04**, **UX-05** y **UX-06**.

## What Changes

- Nuevo componente `console/components/tui.sh` con lo mínimo para dibujar en un terminal,
  en Bash puro y con secuencias VT100 en crudo:
  - **Tamaño**: filas y columnas, con `stty size` y sus retrocesos.
  - **Cursor**: ocultar, mostrar, mover a una posición, guardar y restaurar.
  - **Pantalla alternativa**: entrar y salir sin tocar el contenido previo del terminal.
  - **Teclas**: leer una pulsación, incluidas las flechas y `Esc`, `Enter` y `Ctrl-C`.
  - **Redimensionado**: reaccionar a `SIGWINCH`.
  - **Ceder y recuperar el terminal**: salir de la pantalla alternativa, dejar que otro
    comando escriba con normalidad y volver al panel al terminar.
  - **Restauración garantizada**: al salir, por interrupción o por error, el terminal queda
    como estaba (cursor visible, pantalla principal, eco activado).
- Todas las funciones **no hacen nada** cuando la salida no es un terminal, para que un
  script o un agente que ejecute lo mismo no reciba secuencias de control.

## Non-goals

- **No se construye el TUI.** Esto es la caja de herramientas, no el dibujo.
- No se añade ninguna dependencia. Ni `tput` —10-15 ms por invocación y no aporta
  portabilidad frente a VT100— ni `ncurses`, ni `dialog`, ni `gum`.
- No se sustituyen las funciones de `print_message.sh`: siguen siendo la vía normal de
  escribir mensajes.
- No se toca el contrato de salida ni el modo JSON.

## Capabilities

### New Capabilities
- `terminal-control`: qué puede hacer Dockergento con el terminal —tamaño, cursor,
  pantalla, teclas— y qué garantiza al terminar.

### Modified Capabilities
<!-- Ninguna. -->

## Impact

- **Código**: nuevo `console/components/tui.sh`. Nada más cambia en este cambio: el
  componente nace sin consumidores, y los consumidores llegan en UX-04, UX-05 y TUI-01.
- **Compatibilidad**: debe funcionar en el **Bash 3.2 de macOS**, lo que descarta arrays
  asociativos, `read -N` y `${var@Q}`.
- **Documentación**: `docs/` con las secuencias usadas y por qué, para que quien las lea
  dentro de un año no las cambie por `tput`.
- **Proyectos existentes**: sin impacto. No hay cambio observable hasta que alguien use el
  componente.
- **Dependencias**: ninguna.
