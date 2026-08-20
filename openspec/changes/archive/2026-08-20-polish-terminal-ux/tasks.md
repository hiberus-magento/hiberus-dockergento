## 1. Credenciales sin eco

- [x] 1.1 Leer la contraseña de `transfer-db` con `read -rs` y emitir el salto de línea explícito
- [x] 1.2 Revisar el resto de preguntas del proyecto y anotar si alguna otra pide una credencial
- [x] 1.3 Comprobar que la contraseña no acaba impresa en ningún mensaje ni en la orden que se ejecuta
- [ ] 1.4 Comprobación manual: escribir una contraseña y verificar que no aparece en pantalla ni en el scrollback — **pendiente: requiere teclear en un terminal real; verificado estructuralmente (`read -rs` desactiva el eco por definición) y con un test que impide añadir prompts de credenciales sin `-s`**

## 2. Decisión de color

- [x] 2.1 Implementar la precedencia en `load_colors`: `--no-color` > `NO_COLOR` > `TERM=dumb` > `FORCE_COLOR`/`CLICOLOR_FORCE` > TTY
- [x] 2.2 Añadir `--no-color` a los flags globales y exportar la decisión para los comandos hijo
- [x] 2.3 Verificar que `print_message.sh` no necesita cambios, porque las variables vacías bastan
- [x] 2.4 Comprobar que el contenido de la salida es idéntico con y sin color
- [x] 2.5 Documentar `--no-color` en `data/command_descriptions.json`

## 3. Preguntas que conservan el contexto

- [x] 3.1 Quitar el borrado de pantalla de `custom_question` y de `custom_select`
- [x] 3.2 Separar con una línea en blanco las tablas de requisitos que se redibujan en `version_manager`
- [x] 3.3 Dejar `clear_screen` sin usos, anotada para la biblioteca de componentes (UX-07)
- [ ] 3.4 Comprobación manual de `hm setup`: las preguntas anteriores siguen visibles — **pendiente: requiere responder preguntas en un terminal real; verificado con tests estructurales de que no queda ningún `clear`**

## 4. Verificación

- [x] 4.1 Pruebas de la precedencia de color, una por cada variable y por su combinación con `--no-color`
- [x] 4.2 Prueba de que la salida con y sin color contiene el mismo texto
- [x] 4.3 Prueba de que ningún comando escribe secuencias de color con `NO_COLOR` definida
- [x] 4.4 Verificar en mac y en linux

## 5. Documentación

- [x] 5.1 Documentar en `README.md` las variables de entorno que se respetan y su precedencia
- [x] 5.2 Añadir la entrada al changelog, avisando de que quien tenga `NO_COLOR` notará el cambio
- [x] 5.3 Marcar UX-01, UX-02 y UX-03 en `docs/research/backlog.md` con enlace a este cambio
