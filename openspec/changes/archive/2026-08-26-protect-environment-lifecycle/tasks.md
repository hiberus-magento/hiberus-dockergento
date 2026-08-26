## 1. `hm down -v`

- [x] 1.1 Detectar que la invocación pide borrar volúmenes
- [x] 1.2 Enumerar los volúmenes del proyecto antes de preguntar
- [x] 1.3 Tres respuestas: guardar y destruir, destruir sin guardar, cancelar
- [x] 1.4 Guardar usando el mismo mecanismo que `hm db snapshot`
- [x] 1.5 No preguntar cuando no se piden volúmenes
- [x] 1.6 No preguntar sin terminal ni con la opción global de no preguntar

## 2. `hm stop --snapshot`

- [x] 2.1 Aceptar la opción sin pasarla a Compose
- [x] 2.2 Guardar antes de parar
- [x] 2.3 No parar si la copia falla
- [x] 2.4 Entrada en la ayuda del comando

## 3. `hm docker-stop-all`

- [x] 3.1 Contar los contenedores del proyecto actual y los ajenos por separado
- [x] 3.2 Preguntar antes de parar, diciendo el alcance
- [x] 3.3 No preguntar cuando no hay nada que parar
- [x] 3.4 No preguntar sin terminal ni con la opción global

## 4. Verificación

- [x] 4.1 Test de que `down -v` sin confirmar deja los volúmenes
- [x] 4.2 Test de que aceptar guardar deja una copia restaurable y luego destruye
- [x] 4.3 Test de que destruir sin guardar no crea copia
- [x] 4.4 Test de que `down` sin `-v` no pregunta
- [x] 4.5 Test de que con `--yes` destruye sin preguntar
- [x] 4.6 Test de `stop --snapshot`: la copia existe y el entorno queda parado
- [x] 4.7 Test de que `stop` sin la opción no crea ninguna copia
- [x] 4.8 Test de que `docker-stop-all` sin confirmar no para nada
- [x] 4.9 Test del recuento de contenedores propios y ajenos
- [x] 4.10 Suite completa en mac y linux

## 5. Documentación

- [x] 5.1 `docs/down.md` y `docs/stop.md`: qué preguntan y por qué
- [x] 5.2 `docs/db.md`: enlazar desde las copias al ciclo de vida
- [x] 5.3 Entrada en el changelog de la 1.6.0
- [x] 5.4 Marcar DB-03 en el backlog
