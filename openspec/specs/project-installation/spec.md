# project-installation Specification

## Purpose
TBD - created by archiving change bootstrap-admin-credentials. Update Purpose after archive.
## Requirements
### Requirement: Contraseña de administrador generada

La instalación SHALL crear el administrador con una contraseña generada, distinta en cada
instalación, salvo que se haya fijado una.

#### Scenario: Cada instalación, su contraseña
- **WHEN** se instalan dos proyectos seguidos sin fijar contraseña
- **THEN** cada uno recibe una contraseña distinta

#### Scenario: No se guarda en ningún fichero
- **WHEN** termina una instalación con contraseña generada
- **THEN** la contraseña no aparece en la configuración de la herramienta ni en el proyecto

#### Scenario: Cumple lo que Magento exige
- **WHEN** se genera una contraseña
- **THEN** tiene al menos la longitud mínima que Magento requiere y contiene letras y números

#### Scenario: Sin caracteres que rompan el paso de argumentos
- **WHEN** se genera una contraseña
- **THEN** está formada sólo por letras y dígitos, porque viaja como argumento hasta el contenedor

#### Scenario: Una contraseña fijada se respeta
- **WHEN** la configuración tiene una contraseña con valor
- **THEN** se usa esa y no se genera ninguna

### Requirement: Segundo factor listo al terminar

La instalación SHALL dejar el segundo factor del administrador dado de alta y mostrable, cuando el
módulo de doble factor esté activo.

#### Scenario: Alta y QR
- **WHEN** termina la instalación con el módulo de doble factor activo
- **THEN** el segundo factor del administrador queda dado de alta y se muestra un código QR en el
  terminal para registrarlo en la aplicación del móvil

#### Scenario: Un secreto que la aplicación acepta
- **WHEN** se genera el secreto del segundo factor
- **THEN** es una cadena base32, que es el formato que esperan las aplicaciones de autenticación

#### Scenario: Alternativa al QR
- **WHEN** el código QR no se puede pintar
- **THEN** se muestra la URI de registro, que sirve para el mismo fin

#### Scenario: El módulo desactivado no se activa
- **WHEN** el módulo de doble factor está desactivado en el proyecto
- **THEN** la instalación lo indica y termina correctamente, sin activarlo

### Requirement: Resumen al terminar

La instalación SHALL mostrar al final, en un solo bloque, lo que hace falta para entrar por
primera vez.

#### Scenario: Qué se muestra
- **WHEN** termina la instalación
- **THEN** se muestran la dirección de la tienda, la del panel, el usuario y la contraseña

#### Scenario: Una sola vez
- **WHEN** se muestra la contraseña
- **THEN** aparece al final, después de la salida de los comandos de instalación

