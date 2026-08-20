## 1. Esqueleto

- [ ] 1.1 Crear `console/commands/tui.sh` y eximirlo de la validación de proyecto
- [ ] 1.2 Negarse con un mensaje accionable cuando la salida no es un terminal
- [ ] 1.3 Entrar en la pantalla alternativa y garantizar la restauración al salir, por cualquier vía
- [ ] 1.4 Bucle principal: leer tecla, actuar, redibujar
- [ ] 1.5 Redibujar al recibir `SIGWINCH`

## 2. Vista de flota

- [ ] 2.1 Dibujar el marco y la línea de estado antes de tener datos
- [ ] 2.2 Cargar la flota desde `hm list --json` y dibujar una fila por entorno
- [ ] 2.3 Marcar worktrees, huérfanos y entornos sin metadatos
- [ ] 2.4 Cargar los avisos desde `hm doctor --json` y mostrarlos arriba
- [ ] 2.5 Mostrar de cuándo son los datos y permitir refrescar con una tecla
- [ ] 2.6 Mensaje útil cuando no hay entornos
- [ ] 2.7 Recortar la ruta antes que el nombre y el estado en terminales estrechos

## 3. Vista de detalle

- [ ] 3.1 Abrir el entorno seleccionado y cargar `hm describe --json`
- [ ] 3.2 Mostrar URLs, versión de Magento y estado por servicio
- [ ] 3.3 Volver a la flota

## 4. Acciones

- [ ] 4.1 Salir de la pantalla alternativa, ejecutar el comando de la CLI y volver al panel
- [ ] 4.2 Implementar arrancar, parar, reiniciar, logs y abrir en el navegador
- [ ] 4.3 Propagar el resultado: si la acción falla, el usuario ve el error completo
- [ ] 4.4 Ejecutar la acción en el directorio del entorno, para que las protecciones de la CLI se apliquen
- [ ] 4.5 Comprobar que ninguna acción disponible destruye datos

## 5. Navegación

- [ ] 5.1 Moverse con flechas y con `j`/`k`, abrir con `Enter`, volver con `Esc`, salir con `q`
- [ ] 5.2 Teclas de acción con su inicial, y `?` para la ayuda
- [ ] 5.3 Línea inferior con las teclas del contexto actual
- [ ] 5.4 `Ctrl-C` sale y restaura

## 6. Verificación

- [ ] 6.1 Prueba de que con la salida canalizada no se emite ninguna secuencia de control y el comando falla
- [ ] 6.2 Prueba con pseudo-terminal: abrir, salir y comprobar que el contenido previo del terminal sigue ahí
- [ ] 6.3 Prueba de que el terminal queda restaurado tras una interrupción
- [ ] 6.4 Prueba de las funciones puras de presentación: filas de la flota a partir de un JSON de mentira, incluido el recorte por anchura
- [ ] 6.5 Prueba de que una acción prohibida por la CLI se rechaza igualmente desde el panel
- [ ] 6.6 Comprobación manual con los entornos reales de la máquina
- [ ] 6.7 Verificar en mac y en linux

## 7. Documentación

- [ ] 7.1 Crear `docs/tui.md` con las teclas y lo que hace cada una
- [ ] 7.2 Añadir el comando a `data/command_descriptions.json`, en el grupo de entorno
- [ ] 7.3 Añadir la entrada al changelog
- [ ] 7.4 Marcar TUI-01 en `docs/research/backlog.md` con enlace a este cambio
