## 1. Esqueleto

- [x] 1.1 Crear `console/commands/tui.sh` y eximirlo de la validación de proyecto
- [x] 1.2 Negarse con un mensaje accionable cuando la salida no es un terminal
- [x] 1.3 Entrar en la pantalla alternativa y garantizar la restauración al salir, por cualquier vía
- [x] 1.4 Bucle principal: leer tecla, actuar, redibujar
- [x] 1.5 Redibujar al recibir `SIGWINCH`

## 2. Vista de flota

- [x] 2.1 Dibujar el marco y la línea de estado antes de tener datos
- [x] 2.2 Cargar la flota desde `hm list --json` y dibujar una fila por entorno
- [x] 2.3 Marcar worktrees, huérfanos y entornos sin metadatos
- [x] 2.4 Cargar los avisos desde `hm doctor --json` y mostrarlos arriba
- [x] 2.5 Mostrar de cuándo son los datos y permitir refrescar con una tecla
- [x] 2.6 Mensaje útil cuando no hay entornos
- [x] 2.7 Recortar la ruta antes que el nombre y el estado en terminales estrechos

## 3. Vista de detalle

- [x] 3.1 Abrir el entorno seleccionado y cargar `hm describe --json`
- [x] 3.2 Mostrar URLs, versión de Magento y estado por servicio
- [x] 3.3 Volver a la flota

## 4. Acciones

- [x] 4.1 Salir de la pantalla alternativa, ejecutar el comando de la CLI y volver al panel
- [x] 4.2 Implementar arrancar, parar, reiniciar, logs y abrir en el navegador
- [x] 4.3 Propagar el resultado: si la acción falla, el usuario ve el error completo
- [x] 4.4 Ejecutar la acción en el directorio del entorno, para que las protecciones de la CLI se apliquen
- [x] 4.5 Comprobar que ninguna acción disponible destruye datos

## 5. Navegación

- [x] 5.1 Moverse con flechas y con `j`/`k`, abrir con `Enter`, volver con `Esc`, salir con `q`
- [x] 5.2 Teclas de acción con su inicial, y `?` para la ayuda
- [x] 5.3 Línea inferior con las teclas del contexto actual
- [x] 5.4 `Ctrl-C` sale y restaura

## 6. Verificación

- [x] 6.1 Prueba de que con la salida canalizada no se emite ninguna secuencia de control y el comando falla
- [x] 6.2 Prueba con pseudo-terminal: abrir, salir y comprobar que el contenido previo del terminal sigue ahí
- [x] 6.3 Prueba de que el terminal queda restaurado tras una interrupción
- [x] 6.4 Prueba de las funciones puras de presentación: filas de la flota a partir de un JSON de mentira, incluido el recorte por anchura
- [x] 6.5 Prueba de que una acción prohibida por la CLI se rechaza igualmente desde el panel
- [x] 6.6 Comprobación manual con los entornos reales de la máquina
- [x] 6.7 Verificar en mac y en linux

## 7. Documentación

- [x] 7.1 Crear `docs/tui.md` con las teclas y lo que hace cada una
- [x] 7.2 Añadir el comando a `data/command_descriptions.json`, en el grupo de entorno
- [x] 7.3 Añadir la entrada al changelog
- [x] 7.4 Marcar TUI-01 en `docs/research/backlog.md` con enlace a este cambio
