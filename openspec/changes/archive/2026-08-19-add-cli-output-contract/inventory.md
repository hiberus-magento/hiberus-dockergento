# Inventario previo (tareas 1.1 – 1.3)

## 1.1 Puntos de entrada interactiva

Bloqueantes (esperan a que alguien escriba) — son los que debe cubrir el modo no interactivo:

| Fichero | Líneas | Qué pregunta |
|---|---|---|
| `console/components/input_info.sh` | 29, 192 | `_custom_read` (base de `custom_question`, `custom_select` y `confirm`) y la versión de Magento |
| `console/tasks/version_manager.sh` | 100 | Versión de Magento cuando no hay equivalencia |
| `console/tasks/ai_wizard.sh` | 72, 208, 220, 235, 243 | Selección de plataformas, repos y ramas |
| `console/commands/ai-reset.sh` | 288 | Confirmación de borrado |
| `console/commands/masquerade.sh` | 8 | Confirmación de anonimización |
| `console/commands/transfer-db.sh` | 56-91, 139-164 | 14 preguntas de conexión y de post-proceso |
| `console/commands/transfer-media.sh` | 29-31, 43 | Datos de SSH y "pulsa una tecla" |

No bloqueantes (leen de un flujo o parten una cadena, se dejan como están): todos los
`IFS=',' read -ra` y `while IFS= read -r` de `ai-init`, `ai-pull`, `ai-reset`,
`compatibility`, `ai_extract`, `ai_wizard` y `version_manager`.

Detección de TTY existente: sólo `console/commands/mysql.sh:70`, ya corregida en la 1.4.5.

## 1.2 Clasificación de los comandos

**De flujo de datos** — su stdout es el dato, nunca se envuelve:
`mysqldump`, `mysql` (con `-q` y con importación por stdin), `copy-from-container`,
`copy-to-container`, `docker-compose`.

**De paso** — reenvían la salida de un proceso hijo:
`exec`, `bash`, `magento`, `composer`, `npm`, `n98-magerun`, `grunt`, `test-unit`,
`test-integration`, `cloud`, `cloud-login`, `masquerade`.

**Informativos** — respetan `--json` con envoltura completa:
`compatibility` (y los futuros `describe`, `list`, `doctor`, `version`).

**De control** — ciclo de vida; los mensajes de progreso van a stderr en modo JSON y se
emite una envoltura mínima de resultado al terminar:
`start`, `stop`, `restart`, `rebuild`, `down`, `setup`, `install`, `create-project`,
`purge`, `debug-on`, `debug-off`, `varnish-on`, `varnish-off`, `set-host`, `ssl`,
`update`, `transfer-db`, `transfer-media`, `docker-stop-all`, `ai-init`, `ai-pull`,
`ai-reset`.

## 1.3 Mapa de `exit 1` → código de salida

43 ocurrencias en 21 ficheros.

| Origen | Nuevo código |
|---|---|
| `console/helpers/docker.sh:9` (`is_docker_service_running`) | 3 · docker no disponible |
| `console/helpers/docker.sh:24` (`is_run_service`) | 5 · servicio no levantado |
| `console/tasks/validate_docker_compose.sh:16` | 4 · proyecto no configurado |
| `console/tasks/version_manager.sh:126` (sin `composer.lock`) | 4 · proyecto no configurado |
| `bin/run:66` (comando no encontrado) | 2 · argumentos inválidos |
| Bloques `?)` de `getopts` (`mysql`, `setup`, `install`, `start`, `ssl`, `transfer-*`, `ai-*`) | 2 · argumentos inválidos |
| Validación de argumentos en `ai-init`, `ai-pull`, `ai-reset`, `copy-*`, `create-project` | 2 · argumentos inválidos |
| `console/commands/composer.sh:47` (`create-project` no soportado) | 2 · argumentos inválidos |
| `console/commands/ssl.sh:26,44,56` (mkcert ausente o hitch parado) | 1 · error genérico y 5 según el caso |
| `console/tasks/set_machine_specific_properties.sh:27` (SO no soportado) | 1 · error genérico |
| `console/tasks/validate_bind_mounts.sh:15`, `fix_linux_permissions.sh:18,30` | 1 · error genérico |
