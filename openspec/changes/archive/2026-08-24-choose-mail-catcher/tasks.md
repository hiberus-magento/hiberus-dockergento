## 1. La imagen

- [x] 1.1 `Dockerfiles/mailpit/1.0/Dockerfile` partiendo de la imagen oficial, con versión fijada
- [x] 1.2 Configurarlo para desarrollo: aceptar cualquier autenticación SMTP y no exigir TLS
- [x] 1.3 Añadirlo al flujo de publicación de Docker Hub
- [x] 1.4 Documentar que la publicación es manual y quién la hace

## 2. La elección

- [x] 2.1 Propiedad `MAIL_SERVICE` con `mailhog` por defecto
- [x] 2.2 Rechazar como error de uso cualquier valor que no sea `mailhog` o `mailpit`
- [x] 2.3 `hm setup --mail=<servicio>` para responder sin interacción
- [x] 2.4 Preguntarlo en la instalación interactiva, con Mailhog como respuesta por defecto
- [x] 2.5 Guardar la preferencia en las propiedades del proyecto sólo cuando no es la de por defecto

## 3. La plantilla

- [x] 3.1 Marcadores `<mail_service>` y `<mail_version>` en el servicio y en `depends_on`
- [x] 3.2 Alias de red `mailhog` en los dos casos
- [x] 3.3 Resolver ambos marcadores antes de la sustitución, sin lógica en la plantilla
- [x] 3.4 Entrada `mailpit` en `data/requirements.json` para todas las versiones

## 4. Que se vea

- [x] 4.1 `describe` publica la dirección bajo `mail`, y mantiene `mailhog` con el mismo valor
- [x] 4.2 `hm launch --mail`, manteniendo `--mailhog`
- [x] 4.3 Comprobación de doctor: la imagen del capturador se puede obtener

## 5. Verificación

- [x] 5.1 Test de que sin preferencia el compose generado es idéntico al de antes
- [x] 5.2 Test de que con Mailpit el servicio generado es el suyo y conserva el alias
- [x] 5.3 Test de que un valor no admitido es error de uso
- [x] 5.4 Test de extremo a extremo: enviar SMTP al servicio y verlo en su API, con los dos
- [x] 5.5 Test de que la entrega funciona usando el nombre `mailhog` con Mailpit levantado
- [x] 5.6 Test de las claves `mail` y `mailhog` en `describe`
- [x] 5.7 Test de la comprobación de doctor con una imagen inexistente
- [x] 5.8 Suite completa en mac y linux

## 6. Documentación

- [x] 6.1 `docs/mail.md`: qué captura el correo, cómo se elige y cómo se cambia
- [x] 6.2 Actualizar `docs/launch.md` y la documentación de `describe`
- [x] 6.3 Entrada en el changelog de la 1.6.0
- [x] 6.4 Marcar ENV-03 en el backlog
