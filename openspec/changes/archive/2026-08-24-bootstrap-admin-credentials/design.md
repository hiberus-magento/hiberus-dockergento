## Context

Todo lo que hace falta para esto existe ya en el proyecto; el trabajo es unirlo sin inventar nada
y sin añadir dependencias. Antes de escribir una línea se comprobó, sobre un Magento 2.4.9 real:

| Pregunta | Respuesta, y de dónde |
|---|---|
| ¿Cómo se da de alta un segundo factor desde el terminal? | `security:tfa:google:set-secret <usuario> <secreto>`, leído en `Command/GoogleSecret.php` |
| ¿Qué formato de secreto espera? | base32 — `Model/Provider/Engine/Google.php` genera `Base32::encode(...)` sin relleno |
| ¿Hay forma de pintar un QR sin instalar nada? | Sí: `endroid/qr-code` está en `vendor` y trae `Writer\ConsoleWriter`. Probado dentro del contenedor |
| ¿`qrencode` está disponible? | No, ni en el host ni en la imagen |

Ese último punto es el que descarta la vía obvia y hace que la solución esté dentro del
contenedor y no en el host.

## Goals / Non-Goals

**Goals**
- Que instalar deje un administrador usable, con su segundo factor, sin pasos manuales.
- Que la contraseña no viva en ningún fichero.
- Cero dependencias nuevas, en el host o en la imagen.

**Non-Goals**
- No se desactiva el módulo de 2FA, ni se propone hacerlo. Quien lo tenga desactivado sigue como
  está.
- No se toca ningún proyecto instalado: esto ocurre durante `install`.
- No se gestionan más proveedores que Google Authenticator. Duo, Authy y U2F quedan fuera: nadie
  los ha pedido y cada uno tiene su propio alta.

## Decisions

### 1. La contraseña se genera y no se guarda

`data/config.json` vive en el directorio de la herramienta, es común a todos los proyectos y
**guarda lo que se responde**. Escribir ahí una contraseña la convierte en la contraseña por
defecto del siguiente proyecto y la deja en claro en el disco.

Así que la contraseña generada se usa para instalar, se muestra al terminar y no se persiste. La
propiedad sigue existiendo para quien quiera fijar una: si `admin-password` tiene valor, se
respeta.

### 2. Alfanumérica y larga, en lugar de corta y con símbolos

Magento exige al menos 7 caracteres con letras y números. La contraseña generada son 20
caracteres de `[A-Za-z0-9]`, con al menos una letra y un dígito garantizados en lugar de
probables.

Sin símbolos a propósito: la contraseña viaja como argumento de `setup:install` a través de
`docker compose exec`, y cada símbolo es una oportunidad de que una capa de citado la parta. Veinte
caracteres alfanuméricos dan más entropía que doce con símbolos, y ninguna de las dos se va a
teclear a mano.

### 3. El secreto lo generamos nosotros, porque hay que enseñarlo

Magento genera el secreto solo, la primera vez que lo necesita, y **no hay comando para leerlo**.
Sin secreto no hay QR, así que se genera aquí —base32, el alfabeto que espera la app— y se da de
alta con el comando del módulo. La URI `otpauth://` se compone con ese mismo secreto.

### 4. El QR lo pinta el contenedor, no el host

`endroid/qr-code` está en el `vendor` del proyecto y trae un escritor para terminal. Se invoca con
`php -r` dentro del contenedor de PHP: ninguna dependencia nueva y funciona igual en cualquier
máquina del equipo.

Si por lo que sea no se pudiera pintar —una versión de la librería sin ese escritor, un proyecto
sin `vendor`—, se muestra sólo la URI. Es texto, se pega en la app y funciona igual: el QR es
comodidad, no el mecanismo.

### 5. Un módulo desactivado no se activa

Activar `Magento_TwoFactorAuth` supone `setup:upgrade` y cambiar cómo se entra al panel de un
proyecto. Si está desactivado, la instalación lo dice y continúa. Es información, no una decisión
tomada por el usuario.

### 6. El resumen se imprime una vez, al final

La contraseña aparece una sola vez, cuando ya no hay más salida de comandos que la desplace hacia
arriba. Junto a ella el QR, para tener las dos cosas a la vista en el mismo momento en el que se
va a entrar por primera vez.

## Risks / Trade-offs

- **La contraseña se muestra en el terminal**, y queda en el histórico de la sesión. Es un entorno
  local y la alternativa es escribirla en un fichero, que es peor. El documento lo dice.
- **Si se pierde, hay que cambiarla**: no está guardada en ningún sitio. Se documenta el comando
  para hacerlo (`admin:user:create` con el mismo usuario la actualiza).
- **El QR depende de una librería del proyecto.** Está en Magento desde 2.4.0 por el propio módulo
  de 2FA, y si faltara se degrada a la URI.

## Migration Plan

Ninguna. Afecta a instalaciones nuevas.

## Open Questions

Ninguna.
