# El capturador de correo pasa a ser una elección

## Por qué

La plantilla trae **Mailhog**, que **está sin mantenimiento**: su último desarrollo es de hace
años y su repositorio no acepta cambios. Warden migró a **Mailpit** en su v0.15 y DDEV hizo lo
mismo. Nosotros seguimos con Mailhog en todos los proyectos.

Cambiarlo por decreto no es una opción: hay decenas de entornos en marcha que funcionan, y una
migración forzada convierte una mejora en una tarde perdida para todo el departamento. Pero
dejarlo como está tampoco, porque cada proyecto nuevo nace ya con una pieza abandonada.

La salida es que sea una elección: los proyectos nuevos pueden nacer con Mailpit, los existentes
cambian cuando les venga bien, y nadie tiene que hacer nada el día que esto se publique.

## Qué lo hace fácil

Mailpit escucha en los **mismos puertos** que Mailhog: 1025 para SMTP y 8025 para la interfaz.
Comprobado ejecutándolo (v1.31.0), enviando un correo por el 1025 y viéndolo aparecer en su API.
Eso significa que la configuración de Magento no cambia, ni los puertos publicados, ni la URL que
la gente tiene en el navegador.

## Qué cambia

- **Un Dockerfile para Mailpit** y su entrada en el flujo de publicación, para que la imagen viva
  en `hiberusmagento/` como todas las demás.
- **Una propiedad `MAIL_SERVICE`**, con `mailhog` por defecto. `hm setup` la pregunta y admite
  `--mail=mailpit` para responderla sin interacción.
- La plantilla genera el servicio elegido, **con alias de red `mailhog`** en ambos casos: lo que
  un Magento ya instalado tenga configurado como servidor SMTP sigue resolviendo.
- `describe` publica la dirección del correo bajo la clave `mail`, y mantiene `mailhog` con el
  mismo valor para no romper a nadie. `hm launch --mail` abre la que haya.
- Documentado cómo cambia de servicio un proyecto que ya está en marcha: una propiedad,
  `hm setup -f` y `hm rebuild`.

## Qué no cambia

- **Ningún proyecto existente cambia de capturador.** El valor por defecto sigue siendo Mailhog,
  y quien no toque nada no nota nada.
- Los puertos, la URL y la configuración SMTP de Magento son los mismos con cualquiera de los dos.
- `hm launch --mailhog` sigue funcionando.

## Dependencia externa

La imagen `hiberusmagento/mailpit` **se publica fuera de esta sesión**, a mano. Hasta que exista,
elegir Mailpit deja el proyecto apuntando a una imagen que Docker no puede descargar: la CLI lo
detecta y lo dice antes de dejar el entorno a medias.

## Cómo se sabrá que funciona

- Un proyecto nuevo puede nacer con cualquiera de los dos, y en ambos casos el correo que envía
  Magento aparece en la interfaz del 8025.
- Un proyecto existente que no toca nada sigue con Mailhog, byte a byte.
- Con Mailpit, un Magento configurado contra el servidor `mailhog` sigue entregando correo.
- Elegir un servicio cuya imagen no existe falla con un mensaje que lo explica.
