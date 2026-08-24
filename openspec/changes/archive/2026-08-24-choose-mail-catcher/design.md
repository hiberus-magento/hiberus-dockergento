## Context

El servicio de correo es la pieza más fácil de sustituir del stack —captura SMTP y lo enseña por
web, nada más— y aun así tiene tres ataduras que hay que respetar: el **nombre del servicio**, que
es el servidor SMTP que Magento tiene guardado en su configuración; el **puerto 8025**, que la
gente tiene en marcadores; y el `depends_on` de PHP.

La primera es la que decide el diseño. Un Magento instalado tiene `mailhog` como servidor SMTP en
su base de datos o en su `env.php`. Renombrar el servicio a `mailpit` deja ese correo sin destino,
y el fallo aparece cuando alguien intenta recuperar una contraseña, no al arrancar.

## Goals / Non-Goals

**Goals**
- Que un proyecto nuevo pueda nacer con Mailpit.
- Que uno existente cambie cuando quiera, con una propiedad y un `rebuild`.
- Que quien no haga nada no note nada.

**Non-Goals**
- No se migra ningún proyecto. No hay conversión automática ni aviso recurrente.
- No se generaliza todavía a «servicios opcionales» (ENV-04). Esto es un servicio con dos
  implementaciones, no un catálogo.
- No se publica la imagen: eso ocurre fuera, a mano.

## Decisions

### 1. Los dos servicios responden al nombre `mailhog`

El servicio se llama como la herramienta elegida —`mailhog` o `mailpit`, que es lo que se quiere
leer en `hm describe`, en el panel y en `hm logs`—, y **ambos declaran el alias de red
`mailhog`**.

Con Mailhog el alias es su propio nombre y no hace nada. Con Mailpit es lo que mantiene entregando
el correo de un Magento que fue instalado apuntando a `mailhog`, sin tocar su configuración.

Es una línea en la plantilla y evita el único fallo silencioso posible de este cambio.

### 2. La elección es una propiedad, no un comando

`MAIL_SERVICE` vive donde vive el resto de la configuración del proyecto, con `mailhog` por
defecto. Cambiar de servicio es cambiar esa propiedad y regenerar, igual que se cambia la versión
de PHP: no hace falta un `hm mail switch` que haga lo mismo con más superficie.

`hm setup` la pregunta, y `--mail=<mailhog|mailpit>` la responde sin interacción, como el resto de
opciones de instalación.

### 3. La plantilla no aprende a decidir

El generador de la plantilla es un `sed` de sustituciones, y así se queda. Los dos puntos
variables —el nombre del servicio y su imagen— son dos marcadores más:
`<mail_service>` y `<mail_version>`, resueltos antes de la sustitución.

Meter condicionales en la plantilla obligaría a cambiar el generador a algo con lógica, que es
mucho precio por un `if`.

### 4. Las direcciones se publican bajo `mail`, y `mailhog` se mantiene

`describe` publicaba la URL bajo la clave `mailhog`. Con dos implementaciones posibles esa clave
pasa a llamarse `mail`, y **`mailhog` se sigue publicando con el mismo valor**: cualquier script,
el panel o `hm launch --mailhog` siguen funcionando sin cambios.

Publicar las dos claves durante una versión cuesta una línea de `jq` y evita coordinar un cambio
de nombre con todo el que consuma el JSON.

### 5. Elegir una imagen que no existe se detecta pronto

Mientras `hiberusmagento/mailpit` no esté publicada, elegirla deja el proyecto apuntando a algo
que Docker no puede descargar. El fallo natural sería un `docker compose up` a medias.

`hm doctor` gana una comprobación: si la imagen del servicio de correo no se puede resolver, lo
dice y explica que la publicación es manual. Es la diferencia entre un error de Docker y saber qué
ha pasado.

### 6. La imagen se construye desde la oficial

`Dockerfiles/mailpit/1.0/Dockerfile` parte de `axllent/mailpit`, la imagen oficial del proyecto, y
fija la versión. Compilar Mailpit desde fuente —como se hace hoy con Mailhog, con Go dentro de un
Alpine— no aporta nada: Mailpit publica imágenes multiarquitectura mantenidas.

Se configura para desarrollo: acepta cualquier autenticación SMTP y no exige TLS, porque un
Magento local manda correo con credenciales de mentira.

## Risks / Trade-offs

- **La imagen no existe hasta que alguien la publique.** Es el precio de mantener todo bajo
  `hiberusmagento/`; se mitiga con la comprobación del doctor y quedando documentado.
- **Un proyecto que regenera su compose cambia el nombre del servicio** si además cambia de
  capturador. `hm logs mailhog` pasaría a ser `hm logs mailpit`. El alias cubre la red, no el
  nombre del servicio en Compose, y eso es visible a propósito: lo que corre es otra cosa.
- **Dos claves para la misma URL** en el JSON durante una versión. Es deuda deliberada y con
  fecha: se retira cuando se retire Mailhog.

## Migration Plan

Ninguna obligatoria. Un proyecto que quiera cambiar:

```bash
# en config/docker/properties.json: "MAIL_SERVICE": "mailpit"
hm setup -f
hm rebuild
```

Los correos capturados hasta ese momento se pierden, porque estaban en la memoria del contenedor
anterior. Ninguno de los dos servicios los persiste.

## Open Questions

Ninguna. Cuándo se convierte Mailpit en el valor por defecto es una decisión posterior, y depende
de cuántos proyectos lo hayan probado.
