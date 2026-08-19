## 1. Definición de las etiquetas

- [x] 1.1 Fijar la lista definitiva de etiquetas y su significado, y documentarla en el propio cambio
- [x] 1.2 Decidir el valor por defecto de `hm.profile` mientras no existan perfiles (propuesta: `full`)

## 2. Plantilla de compose

- [x] 2.1 Añadir el bloque `labels` con interpolación `${HM_*:-}` a todos los servicios de `docker-compose/docker-compose.template.yml`
- [x] 2.2 Verificar que `docker compose config` no emite avisos con las variables sin definir
- [x] 2.3 Comprobar que los overlays de mac y linux fusionan correctamente y no pierden las etiquetas

## 3. Exportación de variables

- [x] 3.1 Exportar `HM_PROJECT`, `HM_ROOT`, `HM_WORKTREE`, `HM_PROFILE`, `HM_MAGENTO` y `HM_VERSION` desde `bin/run`
- [x] 3.2 Resolver `HM_ROOT` como ruta absoluta del checkout, y `HM_WORKTREE` como vacío cuando no se está en un worktree
- [x] 3.3 Obtener `HM_MAGENTO` de la configuración del proyecto sin coste perceptible en cada invocación (cachear si hace falta)
- [x] 3.4 Permitir que `HM_AGENT` venga del entorno para que lo estampe el flujo de agentes

## 4. Función de descubrimiento

- [x] 4.1 Añadir a `console/helpers/docker.sh` una función que liste entornos por `label=hm.project`
- [x] 4.2 Añadir la resolución de un servicio dentro de un proyecto por etiquetas, sin filtrar por nombre
- [x] 4.3 Implementar el retroceso a las etiquetas estándar de Compose para entornos sin metadatos
- [x] 4.4 Marcar como huérfano el entorno cuyo `hm.root` no exista en disco

## 5. Verificación

- [x] 5.1 Con dos proyectos distintos levantados, comprobar que el inventario los separa correctamente
- [x] 5.2 Comprobar que un contenedor ajeno a Dockergento no aparece en el inventario
- [x] 5.3 Comprobar que cambiar de rama no altera las etiquetas y que la rama se sigue derivando bien
- [x] 5.4 Comprobar el camino de retroceso con un entorno creado antes del cambio
- [x] 5.5 Verificar en mac y en linux

## 6. Documentación

- [x] 6.1 Documentar las etiquetas y su significado en `docs/`
- [x] 6.2 Documentar en el changelog que `hm setup -f` incorpora los metadatos a proyectos existentes
- [x] 6.3 Marcar ENV-02 como `spec` en `docs/research/backlog.md` con enlace a este cambio
