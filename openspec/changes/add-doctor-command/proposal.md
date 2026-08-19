## Why

Cuando algo va mal, Dockergento dice siempre lo mismo: *"Docker is not properly configured
or docker is not running. Please execute: hm setup"*. Ese único mensaje sale igual si el
demonio de Docker está parado, si falta el `docker-compose.yml`, si el YAML es inválido, si
hay un puerto ocupado por otro proyecto o si el certificado ha caducado. El resultado es
que cada incidencia acaba en soporte interno y se resuelve a base de preguntar.

El diagnóstico es además el punto donde más se nota la falta de multi-proyecto: hoy el
motivo más frecuente de que un entorno no arranque es que **otro proyecto tiene ocupado el
puerto 80, 443 o 3306**, y la CLI no lo dice.

Backlog: **CLI-04**.

## What Changes

- Nuevo comando `hm doctor [--json]` que ejecuta una batería de comprobaciones y devuelve,
  por cada una, un estado (correcto, aviso o error), una explicación y **una acción
  concreta**.
- Comprobaciones incluidas: demonio de Docker, versión de Docker Compose, recursos
  disponibles, puertos requeridos y **qué proceso los ocupa**, validez de la configuración
  de Compose, presencia y coherencia de las propiedades del proyecto, certificados y
  confianza del sistema, entradas de `/etc/hosts`, estado de cada servicio del proyecto y
  espacio ocupado por volúmenes e imágenes.
- Distinción entre comprobaciones **globales** (de la máquina) y **de proyecto**, para que
  el comando sea útil también fuera de un proyecto.
- Código de salida distinto de cero si alguna comprobación termina en error, para poder
  usarlo como paso previo en scripts y en CI.

## Non-goals

- **No repara nada.** No hay `--fix` en esta iteración: diagnosticar y proponer la acción,
  nunca ejecutarla por su cuenta.
- No sustituye a los mensajes de error de los demás comandos; los mejora indirectamente al
  poder remitir a `hm doctor`.
- No comprueba la salud de la aplicación Magento (índices, cachés, cron): eso es otra capa.
- No requiere permisos de administrador para ninguna comprobación.

## Capabilities

### New Capabilities
- `environment-diagnostics`: qué comprueba Dockergento sobre la máquina y sobre el proyecto,
  cómo clasifica los resultados y qué acción propone para cada problema.

### Modified Capabilities
<!-- Ninguna. -->

## Impact

- **Código**: nuevo `console/commands/doctor.sh` y un conjunto de comprobaciones en
  `console/tasks/doctor/`; `console/helpers/docker.sh` y `console/helpers/version.sh` para
  reutilizar las comprobaciones existentes.
- **Datos**: `data/command_descriptions.json`; posible fichero con la lista de puertos
  requeridos.
- **Documentación**: `docs/doctor.md`.
- **Proyectos existentes**: no requiere migración; es un comando nuevo de sólo lectura.
- **Dependencias**: ninguna nueva. Usa herramientas ya presentes en el sistema (`lsof` en
  mac, `ss` o `lsof` en linux) y degrada con elegancia si no están.
