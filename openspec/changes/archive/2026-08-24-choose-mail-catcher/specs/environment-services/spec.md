## ADDED Requirements

### Requirement: Capturador de correo elegible

Un proyecto SHALL poder usar Mailhog o Mailpit como capturador de correo, sin que la elección
afecte a cómo Magento envía ni a cómo se consulta el buzón.

#### Scenario: Por defecto no cambia nada
- **WHEN** un proyecto no expresa ninguna preferencia
- **THEN** usa Mailhog, igual que antes de existir esta elección

#### Scenario: Elegir en la instalación
- **WHEN** se instala un proyecto indicando Mailpit
- **THEN** el entorno generado levanta Mailpit y captura el correo que envía Magento

#### Scenario: La misma puerta para los dos
- **WHEN** el entorno usa cualquiera de los dos
- **THEN** el SMTP escucha en el puerto 1025 y la interfaz web se publica en el 8025

#### Scenario: Un Magento ya instalado sigue entregando
- **WHEN** el proyecto pasa a Mailpit y Magento tiene configurado `mailhog` como servidor SMTP
- **THEN** el correo se sigue entregando, porque el servicio responde también a ese nombre

#### Scenario: Cambiar en un proyecto en marcha
- **WHEN** un proyecto existente cambia su preferencia y regenera la configuración
- **THEN** el entorno pasa a usar el otro capturador sin tocar la configuración de Magento

#### Scenario: Una elección que no existe
- **WHEN** se indica un capturador que no está soportado
- **THEN** el comando falla como error de uso y nombra los admitidos

### Requirement: La dirección del buzón se publica igual con cualquiera de los dos

La introspección SHALL informar de la dirección del buzón sin que quien la consulta tenga que
saber qué implementación hay detrás.

#### Scenario: Clave estable
- **WHEN** se consulta la información del proyecto
- **THEN** la dirección del buzón aparece bajo una clave que no depende de la implementación

#### Scenario: Compatibilidad con lo publicado antes
- **WHEN** algo consume la clave que existía hasta ahora
- **THEN** sigue encontrando la misma dirección

#### Scenario: Abrir el buzón
- **WHEN** se pide abrir el buzón desde la CLI
- **THEN** se abre el del capturador que el proyecto tenga configurado

### Requirement: Una imagen que no se puede obtener se detecta antes de usarla

El diagnóstico SHALL avisar cuando la imagen del capturador configurado no está disponible.

#### Scenario: Imagen no publicada todavía
- **WHEN** el proyecto está configurado con un capturador cuya imagen no existe en el registro
- **THEN** el diagnóstico lo señala y explica que esa imagen se publica manualmente

#### Scenario: Imagen disponible
- **WHEN** la imagen existe, en el registro o ya descargada
- **THEN** el diagnóstico no dice nada al respecto
