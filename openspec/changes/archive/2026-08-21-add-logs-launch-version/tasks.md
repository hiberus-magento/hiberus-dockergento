## 1. `hm logs`

- [x] 1.1 Crear `console/commands/logs.sh` como envoltorio de los registros de Compose
- [x] 1.2 Declararlo comando transparente, para que sus opciones no las interprete el router
- [x] 1.3 Validar los nombres de servicio y fallar nombrando los disponibles
- [x] 1.4 No validar nada cuando no se nombra ningún servicio
- [x] 1.5 Entrada en `data/command_descriptions.json`, grupo de entorno, con sus opciones

## 2. `hm launch`

- [x] 2.1 Crear `console/commands/launch.sh` tomando las URLs de la misma fuente que `describe`
- [x] 2.2 Destinos `--admin`, `--mailhog`, `--rabbitmq`, `--search`; por defecto la tienda
- [x] 2.3 Rechazar dos destinos como error de uso
- [x] 2.4 Abrir con `open` o `xdg-open`; sin lanzador, escribir la dirección
- [x] 2.5 Sin terminal o con `--json`, escribir la dirección y no abrir nada
- [x] 2.6 Explicarlo cuando el proyecto no tiene dominio
- [x] 2.7 Entrada en `data/command_descriptions.json`

## 3. `hm version`

- [x] 3.1 Crear `console/commands/version.sh` reutilizando `hm_version_data`
- [x] 3.2 Añadir versión de Docker y de Compose, aprovechando la caché de `version.sh`
- [x] 3.3 Tolerar que Docker no esté: informar de lo que se sepa
- [x] 3.4 Que no exija proyecto
- [x] 3.5 Dejar `hm --version` intacto
- [x] 3.6 Entrada en `data/command_descriptions.json`

## 4. El panel deja de rodear a la CLI

- [x] 4.1 `l` ejecuta `hm logs -f --tail 100`
- [x] 4.2 `o` ejecuta `hm launch` y se elimina la resolución de URL y de lanzador del panel

## 5. Verificación

- [x] 5.1 Test de que `hm logs` de un servicio inexistente falla con el código de servicio y
  nombra los disponibles
- [x] 5.2 Test de que las opciones de `logs` no las consume el router
- [x] 5.3 Test de que `logs` nunca envuelve su salida en JSON
- [x] 5.4 Test de `launch --json`: devuelve la URL y no abre nada
- [x] 5.5 Test de que dos destinos son un error de uso
- [x] 5.6 Test de `launch` sin dominio configurado
- [x] 5.7 Test de que `version` informa de CLI, Docker y Compose en los dos formatos
- [x] 5.8 Test de que `version` funciona fuera de un proyecto
- [x] 5.9 Test de que `hm --version` no ha cambiado
- [x] 5.10 El test de acciones del panel encuentra `logs.sh` y `launch.sh`
- [x] 5.11 Comprobación manual sobre un entorno real, y suite en mac y linux

## 6. Documentación

- [x] 6.1 `docs/logs.md`, `docs/launch.md`, `docs/version.md`
- [x] 6.2 Actualizar `docs/tui.md` con las acciones reales
- [x] 6.3 Entrada en el changelog
- [x] 6.4 Marcar CLI-05, CLI-06 y CLI-07 en el backlog
