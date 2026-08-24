# Un administrador que se puede usar el primer día

## Por qué

Instalar un proyecto deja hoy dos cabos sueltos, y los dos se pagan en el mismo minuto: el
primer intento de entrar al panel.

**La contraseña es fija y compartida.** Sale de `data/config.json`, en el directorio de la
herramienta, con un valor por defecto igual para todo el departamento. Ese fichero además
**guarda lo que se responde**, así que la contraseña del último proyecto instalado queda escrita
en claro y se hereda como valor por defecto en el siguiente.

**El 2FA no está resuelto.** Magento 2.4 exige segundo factor en el panel desde la instalación, y
sin una forma de darlo de alta desde el terminal la única salida práctica es desactivar el módulo
a mano. Es lo que se ha hecho: en uno de los proyectos de la máquina, `Magento_TwoFactorAuth`
está desactivado. Funciona, y deja el entorno local distinto de producción justo en la parte de
seguridad.

Warden resuelve las dos cosas en la instalación: contraseña aleatoria y el QR de Google
Authenticator pintado en el terminal.

## Qué lo hace posible sin añadir dependencias

Las dos piezas que hacían falta ya están:

- `bin/magento security:tfa:google:set-secret <usuario> <secreto>` da de alta el segundo factor y
  activa el proveedor. Verificado leyendo el código del módulo, no supuesto.
- **`endroid/qr-code` viene en el `vendor` de cualquier Magento** y trae un `ConsoleWriter`.
  Comprobado: pinta un QR escaneable en el terminal, dentro del contenedor, sin instalar nada.

## Qué cambia

- **Contraseña aleatoria por defecto**, generada en el momento, mostrada una vez al terminar y
  **no escrita en ningún fichero**. Quien quiera fijarla sigue pudiendo.
- **El segundo factor se da de alta durante la instalación** y se pinta su QR en el terminal,
  listo para escanear con el móvil. Debajo, la URI `otpauth://` por si se prefiere pegarla.
- Si el módulo de 2FA está desactivado, se dice y se sigue: no se activa nada por sorpresa.
- Al terminar, un resumen con la tienda, el panel, el usuario y la contraseña.

## Qué no cambia

- Instalar con `-u` (usar valores guardados) sigue sin preguntar nada.
- Quien tenga una contraseña fija en `data/config.json` la sigue usando.
- Ningún proyecto ya instalado se toca.

## Cómo se sabrá que funciona

- Dos instalaciones seguidas dan contraseñas distintas.
- La contraseña generada no aparece en `data/config.json` después de instalar.
- La contraseña cumple lo que Magento exige: longitud y mezcla de letras y números.
- El secreto que se da de alta es un base32 válido, que es lo que la app del móvil espera.
- Con el módulo desactivado, la instalación termina bien y avisa.
