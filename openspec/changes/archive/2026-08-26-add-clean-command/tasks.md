## 1. Qué es recogible

- [x] 1.1 Entornos con etiqueta `hm.project` cuyo `hm.root` ya no existe
- [x] 1.2 Excluir siempre los que tienen directorio, parados o en marcha
- [x] 1.3 Volúmenes de esos proyectos, atribuidos por sus contenedores
- [x] 1.4 Volúmenes sin contenedores que los expliquen: aparte, como no atribuibles
- [x] 1.5 Entornos sin etiquetas: no atribuibles

## 2. Informar

- [x] 2.1 Recuento por entorno: contenedores y volúmenes
- [x] 2.2 Sección aparte para lo no atribuible, con nombres
- [x] 2.3 Imágenes sueltas y caché: cuánto ocupan y qué comando de Docker las limpia
- [x] 2.4 Salida JSON con las dos listas separadas
- [x] 2.5 Mensaje útil cuando no hay nada que recoger

## 3. Borrar

- [x] 3.1 Sólo con `--force`
- [x] 3.2 Calcular el espacio a liberar antes de preguntar, sólo en este camino
- [x] 3.3 Confirmación, saltable con la opción global
- [x] 3.4 Eliminar contenedores y después volúmenes
- [x] 3.5 No tocar nunca lo no atribuible

## 4. Verificación

- [x] 4.1 Test de que sin opciones no se elimina nada
- [x] 4.2 Test de que un proyecto con directorio nunca aparece como recogible
- [x] 4.3 Test de que un proyecto sin directorio sí, con sus volúmenes
- [x] 4.4 Test de que los volúmenes no atribuibles se listan y no se borran con `--force`
- [x] 4.5 Test de que no confirmar no borra
- [x] 4.6 Test de que con `--force --yes` borra lo recogible y sólo eso
- [x] 4.7 Test de que las copias de base de datos sobreviven
- [x] 4.8 Test de que el comando no invoca ningún `prune` de Docker
- [x] 4.9 Suite completa en mac y linux

## 5. Documentación

- [x] 5.1 `docs/clean.md`, incluyendo el límite de los volúmenes no atribuibles
- [x] 5.2 Entrada en `data/command_descriptions.json`
- [x] 5.3 Entrada en el changelog de la 1.6.0
- [x] 5.4 Marcar CLI-08 en el backlog
