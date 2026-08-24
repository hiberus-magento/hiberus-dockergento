## 1. La contraseña

- [x] 1.1 Generador: 20 caracteres `[A-Za-z0-9]`, con al menos una letra y un dígito garantizados
- [x] 1.2 Usarla cuando la configuración no fije ninguna
- [x] 1.3 No escribirla en `data/config.json` ni en las propiedades del proyecto
- [x] 1.4 Respetar una contraseña fijada

## 2. El segundo factor

- [x] 2.1 Generador de secreto base32
- [x] 2.2 Detectar si el módulo de doble factor está activo
- [x] 2.3 Dar de alta el secreto con el comando del módulo
- [x] 2.4 Componer la URI `otpauth://` con el dominio del proyecto como emisor
- [x] 2.5 Pintar el QR con la librería del propio proyecto, dentro del contenedor
- [x] 2.6 Degradar a la URI cuando no se pueda pintar
- [x] 2.7 Con el módulo desactivado, informar y continuar

## 3. El resumen

- [x] 3.1 Bloque final con tienda, panel, usuario y contraseña
- [x] 3.2 Mostrarlo después de toda la salida de instalación
- [x] 3.3 Advertir de que la contraseña no queda guardada en ningún sitio

## 4. Verificación

- [x] 4.1 Test de que dos generaciones seguidas no coinciden
- [x] 4.2 Test de longitud, alfabeto y mezcla de letras y números
- [x] 4.3 Test de que el secreto generado es base32 válido
- [x] 4.4 Test de que una contraseña fijada gana
- [x] 4.5 Test de que la contraseña generada no acaba en `data/config.json`
- [x] 4.6 Test del QR: que la librería del proyecto lo pinta, y del respaldo a la URI
- [x] 4.7 Test de la ruta con el módulo desactivado
- [x] 4.8 Suite completa en mac y linux

## 5. Documentación

- [x] 5.1 `docs/install.md`: credenciales, segundo factor y cómo cambiar la contraseña
- [x] 5.2 Entrada en el changelog de la 1.6.0
- [x] 5.3 Marcar INST-01 en el backlog
