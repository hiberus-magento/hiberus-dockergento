## 1. Detección

- [ ] 1.1 Crear `console/helpers/worktree.sh` con la resolución del checkout principal mediante `git rev-parse --path-format=absolute --git-common-dir`
- [ ] 1.2 Añadir el retroceso para versiones de git sin `--path-format`
- [ ] 1.3 Soportar `HM_PROJECT_DIR` como valor forzado
- [ ] 1.4 Devolver "no es worktree" sin ruido cuando el directorio no está bajo control de git
- [ ] 1.5 Comprobar que el checkout principal contiene realmente un proyecto Dockergento antes de considerarlo tal

## 2. Resolución de rutas

- [ ] 2.1 En `bin/run`, resolver `CUSTOM_PROPERTIES_DIR` y `CUSTOM_COMMANDS_DIR` contra el checkout principal cuando se detecte worktree
- [ ] 2.2 Construir `DOCKER_COMPOSE` con rutas absolutas a los ficheros del checkout principal
- [ ] 2.3 Añadir `--project-directory <checkout principal>` a la invocación de Compose
- [ ] 2.4 Verificar con `docker inspect` que, tras un comando permitido, los montajes siguen apuntando al checkout principal

## 3. Guardarraíles

- [ ] 3.1 Definir en `bin/run` la lista explícita de comandos bloqueados en worktree
- [ ] 3.2 Implementar el bloqueo con el nuevo código de salida reservado
- [ ] 3.3 Redactar el mensaje de bloqueo con contexto, consecuencia evitada y alternativas
- [ ] 3.4 Añadir `--force` como opción global de una sola invocación en `console/helpers/process_hm_options.sh`
- [ ] 3.5 Añadir el aviso de "los contenedores sirven el código del checkout principal" en los comandos permitidos que operan sobre código
- [ ] 3.6 Añadir el aviso específico de `composer` en mac por la sincronización de vendor

## 4. Verificación

- [ ] 4.1 Crear un worktree de prueba sobre un proyecto real y comprobar que `hm start`, `hm down -v` y `hm rebuild` quedan bloqueados
- [ ] 4.2 Comprobar que `hm bash`, `hm exec`, `hm magento`, `hm mysql -q` y `hm describe` funcionan desde el worktree contra el entorno principal
- [ ] 4.3 Comprobar con `docker inspect` que ningún comando permitido cambia los montajes
- [ ] 4.4 Comprobar que `--force` ejecuta y que no persiste en la invocación siguiente
- [ ] 4.5 Comprobar que desde el checkout principal el comportamiento es idéntico al anterior
- [ ] 4.6 Verificar en mac y en linux

## 5. Documentación

- [ ] 5.1 Documentar el trabajo con worktrees en `docs/` y en `README.md`
- [ ] 5.2 Explicar en el changelog el comportamiento destructivo que existía antes
- [ ] 5.3 Documentar el nuevo código de salida junto a la tabla del contrato de salida
- [ ] 5.4 Marcar WT-01 como `spec` en `docs/research/backlog.md` con enlace a este cambio
