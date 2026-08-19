## 1. Infraestructura del comando

- [ ] 1.1 Definir el contrato de una comprobación (id, ámbito, severidad, mensaje, acción) y documentarlo
- [ ] 1.2 Crear `console/commands/doctor.sh` con el bucle de ejecución, el resumen final y el cálculo del código de salida
- [ ] 1.3 Añadir límite de tiempo por comprobación y aislamiento de fallos para que ninguna aborte el resto
- [ ] 1.4 Soportar `--json` según el contrato de salida y `--only=<id>` para ejecutar una sola
- [ ] 1.5 Permitir la ejecución fuera de un proyecto, ejecutando sólo el ámbito global

## 2. Comprobaciones globales

- [ ] 2.1 Demonio de Docker disponible
- [ ] 2.2 Versión de Docker Compose soportada
- [ ] 2.3 Puertos requeridos, resolviendo el proyecto Dockergento que los ocupa por etiquetas
- [ ] 2.4 Puertos requeridos ocupados por procesos del host, con `lsof` en mac y `ss`/`lsof` en linux, degradando a aviso si no hay herramienta
- [ ] 2.5 Espacio ocupado por volúmenes e imágenes, con umbral de aviso
- [ ] 2.6 `mkcert` instalado y su CA en el almacén de confianza, con la rama correspondiente a cada plataforma
- [ ] 2.7 En linux, pertenencia del usuario al grupo `docker`; en mac, espacio del disco virtual de Docker Desktop

## 3. Comprobaciones de proyecto

- [ ] 3.1 Existencia y validez de los ficheros de Compose del proyecto
- [ ] 3.2 Coherencia de `config/docker/properties.json` (proyecto, dominio, directorio de Magento)
- [ ] 3.3 Estado de cada servicio del proyecto
- [ ] 3.4 Certificado del dominio presente y no caducado
- [ ] 3.5 Resolución del dominio a local, proponiendo `hm set-host` cuando falte
- [ ] 3.6 Presencia de `composer.lock` y coherencia de la versión de Magento con las imágenes configuradas

## 4. Origen de los datos

- [ ] 4.1 Obtener la lista de puertos requeridos de la configuración y no duplicarla en el código del comando
- [ ] 4.2 Reutilizar `console/helpers/docker.sh` y `console/helpers/version.sh` en lugar de reimplementar comprobaciones

## 5. Verificación

- [ ] 5.1 Provocar cada error de forma controlada (Docker parado, puerto ocupado por otro proyecto, compose inválido, certificado caducado, dominio sin resolver) y comprobar el mensaje y la acción propuesta
- [ ] 5.2 Comprobar que sólo los errores devuelven código distinto de cero
- [ ] 5.3 Comprobar la ejecución fuera de un proyecto
- [ ] 5.4 Medir el tiempo total y confirmar que se mantiene por debajo de 5 segundos
- [ ] 5.5 Comprobar que ninguna comprobación pide privilegios de administrador
- [ ] 5.6 Verificar en mac y en linux

## 6. Documentación

- [ ] 6.1 Crear `docs/doctor.md` con la lista de comprobaciones y su significado
- [ ] 6.2 Añadir el comando a `data/command_descriptions.json`
- [ ] 6.3 Marcar CLI-04 como `spec` en `docs/research/backlog.md` con enlace a este cambio
