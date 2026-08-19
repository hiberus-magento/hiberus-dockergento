## 1. Inventario previo

- [x] 1.1 Listar todos los puntos de entrada interactiva del repositorio (`read -rp`, `read -r`, `[ -t 0 ]`, `custom_question`, `custom_select`, `confirm`) y anotar en qué comandos aparecen
- [x] 1.2 Clasificar los 40 comandos actuales en informativos, de flujo de datos y de paso, según la decisión 2 del diseño, y dejar la lista en el propio cambio
- [x] 1.3 Localizar todos los `exit 1` y mapear cada uno al código de salida que le corresponde

## 2. Componente de salida JSON

- [x] 2.1 Crear `console/components/print_json.sh` con `json_success`, `json_error` y `json_kv`, construyendo siempre con `jq -n --arg`
- [x] 2.2 Emitir `schema_version`, `command` y `ok` en toda respuesta; `data` en éxito y `error` en fallo
- [x] 2.3 Escribir los errores por stderr y los éxitos por stdout
- [x] 2.4 Verificar el escapado con valores que contengan comillas, saltos de línea y UTF-8

## 3. Flags globales y detección de formato

- [x] 3.1 Añadir `--json`, `--no-json` y `--yes` al parseo de opciones globales en `console/helpers/process_hm_options.sh`, sin romper el paso de argumentos a los comandos
- [x] 3.2 Exportar `HM_OUTPUT_FORMAT` (`text` | `json`) desde `bin/run`, resolviendo flag > variable > detección de TTY con `[ -t 1 ]`
- [x] 3.3 Exportar `HM_NON_INTERACTIVE` a partir de `--yes` o de la variable de entorno homónima
- [x] 3.4 Hacer que los comandos de flujo de datos y de paso ignoren `HM_OUTPUT_FORMAT` para su salida principal

## 4. Códigos de salida

- [x] 4.1 Definir las constantes de código de salida en un helper compartido
- [x] 4.2 Aplicar el código 3 en `console/helpers/docker.sh` (`is_docker_service_running`) y el 5 en `is_run_service`
- [x] 4.3 Aplicar el código 4 en `console/tasks/validate_docker_compose.sh` (proyecto no configurado)
- [x] 4.4 Aplicar el código 2 en todos los bloques `?)` de `getopts` (argumentos inválidos)
- [x] 4.5 Revisar el resto de `exit 1` del inventario 1.3 y ajustarlos

## 5. Modo no interactivo

- [x] 5.1 Adaptar `console/components/input_info.sh` para que, con `HM_NON_INTERACTIVE`, use el valor por defecto sin preguntar
- [x] 5.2 Hacer que una pregunta sin valor por defecto falle con código 2 e indique el flag que la evita
- [x] 5.3 Integrar `USE_DEFAULT_SETTINGS` como caso particular del nuevo modo, sin romper el comportamiento actual de `setup` e `install`
- [x] 5.4 Adaptar los asistentes `ai-init`, `ai-pull` y `ai-reset`
- [x] 5.5 Adaptar `transfer-db` y `create-project`

## 6. Silenciar la decoración en modo JSON

- [x] 6.1 Hacer que `print_message.sh` no escriba en stdout la salida decorativa (`print_processing`, `print_info`) cuando `HM_OUTPUT_FORMAT` es `json`
- [x] 6.2 Redirigir a stderr los mensajes de progreso de los comandos de flujo de datos

## 7. Verificación

- [x] 7.1 Script de comprobación que ejecuta cada comando con stdin cerrado, sin TTY y con `HM_NON_INTERACTIVE=1`, y falla si alguno se bloquea o supera un tiempo límite
- [x] 7.2 Comprobar que la salida JSON de cada comando informativo pasa por `jq -e .`
- [x] 7.3 Comprobación manual con un proyecto real: `hm mysqldump` sigue produciendo SQL, `hm mysql -q` sigue devolviendo el resultado y `hm magento cache:clean` sigue mostrando su salida
- [x] 7.4 Comprobación manual de que la experiencia en terminal interactiva es idéntica a la anterior
- [x] 7.5 Verificar el comportamiento en mac y en linux

## 8. Documentación

- [x] 8.1 Documentar los flags globales en `data/command_descriptions.json`
- [x] 8.2 Documentar en `README.md` la convención de formato, la tabla de códigos de salida y el modo no interactivo
- [x] 8.3 Añadir la nota de cambio de códigos de salida al changelog de la versión correspondiente
- [x] 8.4 Marcar CLI-01 como `spec` en `docs/research/backlog.md` con enlace a este cambio
