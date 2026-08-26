## 1. El comando

- [x] 1.1 `hm share`: túnel rápido de Cloudflared en un contenedor, en la red del proyecto
- [x] 1.2 Apuntar al servicio web del proyecto por nombre, no por puerto de la máquina
- [x] 1.3 Reescribir la cabecera `Host` al dominio del proyecto
- [x] 1.4 Esperar y extraer la dirección pública de la salida del túnel
- [x] 1.5 Fallar con un mensaje claro si no se obtiene dirección

## 2. Seguridad

- [x] 2.1 Advertir de que el entorno queda expuesto, nombrando el proyecto
- [x] 2.2 Confirmación, saltable con la opción global
- [x] 2.3 Primer plano: cerrar y limpiar al terminar
- [x] 2.4 `--stop` para recoger lo que quedó abierto
- [x] 2.5 Un `share` nuevo limpia el anterior

## 3. Verificación

- [x] 3.1 Test de extremo a extremo: la dirección pública devuelve el contenido del proyecto
- [x] 3.2 Test de que sin confirmar no se abre nada
- [x] 3.3 Test de que al cerrar no queda ningún contenedor
- [x] 3.4 Test de que se salta si no hay salida a internet, en lugar de fallar
- [x] 3.5 Suite completa en mac y linux

## 4. Documentación

- [x] 4.1 `docs/share.md`, incluyendo qué queda expuesto y qué pasa con `base_url`
- [x] 4.2 Entrada en `data/command_descriptions.json`
- [x] 4.3 Entrada en el changelog de la 1.7.0
- [x] 4.4 Marcar NET-04 en el backlog
