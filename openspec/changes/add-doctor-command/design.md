## Context

Hoy la validación se reduce a dos comprobaciones: `is_docker_service_running` en
`console/helpers/docker.sh` (¿responde `docker info`?) y
`console/tasks/validate_docker_compose.sh` (¿`docker compose config -q` da error?). Ambas
son binarias y comparten un único mensaje de salida, que además propone siempre la misma
acción (`hm setup`), correcta sólo en una parte de los casos.

La lista de puertos que el stack publica está fija en la plantilla: 80, 443, 3306, 9200,
8025, 5672 y 15672. Como no existe proxy compartido, cualquiera de ellos ocupado por otro
proyecto impide arrancar, y ése es hoy el fallo más frecuente.

## Goals / Non-Goals

**Goals:**
- Que el 90 % de las incidencias que hoy llegan a soporte se resuelvan leyendo la salida del
  comando.
- Que cada problema venga con la orden exacta que lo arregla.
- Que sea rápido: si tarda, no se usa.

**Non-Goals:**
- Reparar automáticamente.
- Pedir contraseña de administrador.
- Diagnosticar la aplicación Magento.

## Decisions

### 1. Cada comprobación es una función con un contrato fijo

Un fichero por comprobación en `console/tasks/doctor/`, todas con la misma firma y
devolviendo id, ámbito (global o de proyecto), severidad (ok, warning, error), mensaje y
acción sugerida. Añadir una comprobación nueva es añadir un fichero, sin tocar el comando.

Alternativa descartada: un único script con una cadena de `if`. Más corto de escribir y
peor de mantener; además impide ejecutar comprobaciones sueltas
(`hm doctor --only=ports`), que se quiere permitir desde el principio.

### 2. Ámbitos separados: máquina y proyecto

Fuera de un proyecto, `hm doctor` ejecuta sólo las comprobaciones globales y lo dice, en
lugar de fallar. Es el caso de "acabo de instalar Dockergento, ¿está todo bien?", que hoy
no tiene respuesta.

### 3. Severidades, y sólo `error` afecta al código de salida

`warning` es para lo que conviene arreglar pero no impide trabajar (espacio de volúmenes
alto, certificado próximo a caducar, versión de Compose antigua pero soportada). Sólo
`error` devuelve código distinto de cero, para que `hm doctor && hm start` sea un patrón
válido en scripts.

### 4. Detección de puertos sin privilegios

Para cada puerto requerido: si está libre, correcto; si lo ocupa un contenedor, se resuelve
**qué proyecto** lo tiene mediante el descubrimiento por etiquetas, que es el dato
verdaderamente útil ("lo tiene `otro-proyecto`, párrarlo con `hm stop` allí"); si lo ocupa
un proceso del host, se usa `lsof -nP -iTCP:<puerto> -sTCP:LISTEN` en mac y `ss -ltnp` o
`lsof` en linux. Si ninguna herramienta está disponible, la comprobación devuelve aviso en
vez de error: nunca se pide `sudo`, así que puede que no se vea el nombre del proceso ajeno,
y eso es aceptable.

### 5. Presupuesto de tiempo y aislamiento de fallos

Objetivo: menos de 5 segundos en total. Cada comprobación lleva su propio límite de tiempo
y, si lo supera o falla de forma inesperada, se reporta como aviso con el motivo, pero
**no aborta el resto**. Un `doctor` que revienta a la mitad es peor que no tenerlo.

### 6. Certificados y confianza

Se comprueba que exista el certificado del dominio, que no esté caducado y que `mkcert`
esté instalado y su CA en el almacén de confianza del sistema. La comprobación del almacén
difiere por plataforma (llavero en mac, `/usr/local/share/ca-certificates` y `nss` en
linux), y se implementa como dos ramas de la misma comprobación.

### 7. `/etc/hosts` y dominios

Se verifica que el dominio del proyecto resuelva a local. Es una comprobación de lectura:
si falta la entrada, la acción propuesta es `hm set-host`, nunca escribirla. Esta
comprobación quedará obsoleta cuando exista dnsmasq (NET-03) y se marcará entonces.

### 8. Diferencias mac / linux

Además del almacén de certificados y de la herramienta de puertos, en mac se comprueba el
espacio del disco virtual de Docker Desktop, que es una causa clásica de fallos silenciosos
al escribir; en linux se comprueba que el usuario pertenezca al grupo `docker` y el estado
de los permisos que `fix_linux_permissions.sh` ya trata de arreglar.

## Risks / Trade-offs

- **Falsos positivos que erosionan la confianza** → ante la duda, `warning` y no `error`.
  Una comprobación que se equivoca es peor que una que falta.
- **Se convierte en un cajón de sastre** → el contrato fijo por comprobación y la
  posibilidad de ejecutar una sola mantienen el coste bajo, pero el criterio de admisión
  debe ser explícito: sólo entra lo que impide trabajar y tiene una acción concreta.
- **Coste de mantenimiento al evolucionar el stack** → la lista de puertos debe salir de la
  configuración, no estar duplicada en el código del comando.
- **Herramientas de red ausentes en contenedores o sistemas minimalistas** → degradación a
  aviso, nunca fallo.

## Migration Plan

No aplica: comando nuevo de sólo lectura. Como seguimiento, los mensajes de error existentes
deberían pasar a remitir a `hm doctor`, pero eso es trabajo posterior y no bloquea.

## Open Questions

- ¿Debe `hm doctor` ejecutarse automáticamente cuando otro comando falla? Propuesta: no de
  forma automática, pero sí sugerirlo en el mensaje de error.
- ¿Se incluye una comprobación de la versión de `hm` frente a la última publicada? Requiere
  red y por tanto un límite de tiempo estricto; propuesta: sí, como aviso y saltable con
  `--offline`.
