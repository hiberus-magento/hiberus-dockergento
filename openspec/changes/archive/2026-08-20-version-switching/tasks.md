## 1. Identificación de la versión

- [x] 1.1 Sustituir `git describe --tags --abbrev=0` por la descripción completa en `--version`
- [x] 1.2 Añadir la rama, o el estado desacoplado, y si hay cambios sin guardar
- [x] 1.3 Añadir la ruta de instalación
- [x] 1.4 Soportar `--json` según el contrato de salida, con cada dato en su clave
- [x] 1.5 Comprobar los tres casos: en un tag, por delante de un tag y con cambios sin guardar

## 2. `hm switch`

- [x] 2.1 Crear `console/commands/switch.sh` y eximirlo de la validación de proyecto
- [x] 2.2 Comprobar que la instalación es un clon de git, con un mensaje claro si no
- [x] 2.3 Negarse si hay cambios sin guardar, listando los ficheros afectados
- [x] 2.4 Refrescar tags y ramas remotas antes de resolver la referencia
- [x] 2.5 Cambiar a la referencia indicada y regenerar el autocompletado
- [x] 2.6 Informar de la versión resultante y de dónde leer qué cambió
- [x] 2.7 Implementar `--list` con versiones ordenadas y ramas, marcando la actual
- [x] 2.8 Implementar `--stable`
- [x] 2.9 Fallar sin dejar el checkout a medias cuando la referencia no existe

## 3. Protección de `hm update`

- [x] 3.1 Detectar el checkout desacoplado y negarse a actualizar
- [x] 3.2 Explicar en qué versión está y remitir a `hm switch`
- [x] 3.3 Mantener el comportamiento actual cuando se está en una rama

## 4. Verificación

- [x] 4.1 Pruebas de la descripción de versión en los tres estados, sobre clones de prueba
- [x] 4.2 Prueba de que `switch` se niega con cambios sin guardar y no toca nada
- [x] 4.3 Prueba de que `switch` a una referencia inexistente no cambia el checkout
- [x] 4.4 Prueba de que `update` no hace nada en un checkout desacoplado
- [x] 4.5 Prueba de que `update` sigue funcionando en una rama
- [x] 4.6 **Verificación real de la vuelta atrás**: con un proyecto de verdad, bajar de versión y comprobar que los comandos siguen funcionando y que las cachés de la versión posterior se ignoran
- [x] 4.7 Verificar en mac y en linux

## 5. Documentación

- [x] 5.1 Documentar el modelo de ramas y tags en `README.md`: `main` estable, `feature/*`, `release/X.Y.Z` y tags `X.Y.Z-rc.N` para validación interna
- [x] 5.2 Documentar cómo probar una candidata y cómo volver, con `hm switch`
- [x] 5.3 Crear `docs/switch.md` y añadir el comando a `data/command_descriptions.json`, en el grupo de herramientas
- [x] 5.4 Añadir la entrada al changelog, avisando de que hasta esta versión `hm update` saca de un tag sin decirlo
- [x] 5.5 Marcar REL-01, REL-02 y REL-03 en `docs/research/backlog.md` con enlace a este cambio
