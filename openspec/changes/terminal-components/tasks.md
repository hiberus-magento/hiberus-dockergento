## 1. Tamaño del terminal

- [ ] 1.1 Función que interprete una salida de `stty size` dada como texto, sin tocar el terminal
- [ ] 1.2 Función que obtenga el tamaño real, con retroceso a `$LINES`/`$COLUMNS` y a 24×80
- [ ] 1.3 Actualizar el tamaño al recibir `SIGWINCH`, sin dibujar nada en el manejador

## 2. Cursor y pantalla

- [ ] 2.1 Ocultar y mostrar el cursor
- [ ] 2.2 Mover el cursor a una posición y guardar/restaurar la posición
- [ ] 2.3 Entrar y salir de la pantalla alternativa
- [ ] 2.4 Borrar la línea actual
- [ ] 2.5 Ceder el terminal y recuperarlo, para que un comando externo escriba con normalidad
- [ ] 2.6 Documentar cada secuencia en el propio fichero, con lo que hace y por qué no se usa `tput`

## 3. Restauración garantizada

- [ ] 3.1 Instalar el `trap` sobre `EXIT`, `INT` y `TERM` al entrar en pantalla alternativa u ocultar el cursor
- [ ] 3.2 Restaurar pantalla principal, cursor visible, eco activado y región de scroll completa
- [ ] 3.3 Hacer la restauración idempotente
- [ ] 3.4 No pisar `trap`s que el programa ya tuviera instalados

## 4. Teclado

- [ ] 4.1 Leer una pulsación con `read -rsn1`
- [ ] 4.2 Distinguir flechas de `Esc` con un tiempo de espera corto
- [ ] 4.3 Devolver nombres (`up`, `down`, `enter`, `esc`, `ctrl-c`) en lugar de bytes
- [ ] 4.4 Función pura que traduzca una secuencia dada como texto a un nombre

## 5. Inofensivo sin terminal

- [ ] 5.1 Cada función de emisión no escribe nada cuando la salida no es un terminal
- [ ] 5.2 El tamaño devuelve el valor por defecto en ese caso
- [ ] 5.3 Comprobar que un comando canalizado no recibe ninguna secuencia de control

## 6. Verificación

- [ ] 6.1 Pruebas unitarias de las funciones puras: interpretación del tamaño y de las teclas
- [ ] 6.2 Pruebas de que nada se emite sin terminal
- [ ] 6.3 Prueba con pseudo-terminal (`script`) del camino interactivo: entrar y salir de la pantalla alternativa
- [ ] 6.4 Prueba de ceder el terminal, ejecutar algo y recuperarlo
- [ ] 6.5 Prueba de que el terminal queda restaurado tras una interrupción
- [ ] 6.6 Comprobación manual: entrar en pantalla alternativa, salir y verificar que el scrollback anterior sigue ahí
- [ ] 6.7 Verificar en el Bash 3.2 de macOS y en el Bash 5 de linux

## 7. Documentación

- [ ] 7.1 Documentar el componente en `docs/`, con la tabla de secuencias y el criterio de no usar `tput`
- [ ] 7.2 Añadir la entrada al changelog
- [ ] 7.3 Marcar UX-07 en `docs/research/backlog.md` con enlace a este cambio
