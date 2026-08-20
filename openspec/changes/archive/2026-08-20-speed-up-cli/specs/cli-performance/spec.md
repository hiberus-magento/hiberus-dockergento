## ADDED Requirements

### Requirement: Presupuesto de respuesta de la CLI

Las rutas de uso frecuente SHALL responder dentro de un presupuesto de tiempo medido y
documentado.

#### Scenario: Listado de comandos
- **WHEN** se ejecuta `hm --help` en una máquina de desarrollo con Docker en marcha
- **THEN** la respuesta se completa en menos de 500 ms

#### Scenario: Arranque mínimo
- **WHEN** se ejecuta un comando que no necesita Docker ni el proyecto, como `hm --version`
- **THEN** la respuesta se completa en menos de 400 ms

#### Scenario: Diagnóstico completo
- **WHEN** se ejecuta `hm doctor` dentro de un proyecto
- **THEN** la respuesta se completa en menos de 2 segundos

#### Scenario: Vigilancia de la regresión
- **WHEN** una de esas rutas se pasa de su presupuesto
- **THEN** la batería de pruebas falla indicando qué ruta y con qué tiempo

### Requirement: Coste proporcional al trabajo pedido

Una invocación SHALL evitar el trabajo que su comando no va a usar.

#### Scenario: El listado de comandos no consulta a Docker
- **WHEN** se ejecuta `hm --help`
- **THEN** no se invoca a Docker ni a Docker Compose

#### Scenario: Un comando que no crea contenedores no lee composer.lock
- **WHEN** se ejecuta un comando que no crea ni recrea contenedores
- **THEN** no se lee `composer.lock` para averiguar la versión de Magento

#### Scenario: La detección de Docker Compose ocurre una vez
- **WHEN** una invocación necesita la versión de Docker Compose más de una vez
- **THEN** se consulta a Docker una sola vez

#### Scenario: Un valor calculado no se recalcula
- **WHEN** un valor perezoso ya ha sido calculado en esta invocación
- **THEN** las siguientes lecturas usan el valor memorizado

### Requirement: Las etiquetas del entorno siguen siendo correctas

El cálculo perezoso NO SHALL degradar los metadatos estampados en los contenedores.

#### Scenario: Contenedor recién creado
- **WHEN** se levanta un entorno y se inspeccionan las etiquetas de sus contenedores
- **THEN** `hm.magento` y `hm.version` tienen valor, igual que antes del cambio

#### Scenario: Comando que no crea contenedores
- **WHEN** se ejecuta un comando de sólo lectura
- **THEN** las etiquetas de los contenedores existentes no se modifican

### Requirement: Validación de configuración con caché verificable

La validación de la configuración de Compose SHALL poder reutilizar un resultado anterior,
y SHALL invalidarse cuando la configuración cambie.

#### Scenario: Configuración sin cambios
- **WHEN** se ejecuta un comando y los ficheros de Compose no han cambiado desde la última
  validación correcta
- **THEN** no se invoca a Compose para validar

#### Scenario: Configuración modificada
- **WHEN** un fichero de Compose se ha modificado desde la última validación
- **THEN** la configuración se vuelve a validar

#### Scenario: Configuración que pasa a ser inválida
- **WHEN** la configuración se rompe después de una validación correcta
- **THEN** el siguiente comando detecta el error y falla con el código de proyecto no configurado

#### Scenario: El diagnóstico nunca usa la caché
- **WHEN** se ejecuta `hm doctor`
- **THEN** la validación se ejecuta de verdad, sin consultar la caché

#### Scenario: La caché no toca el repositorio
- **WHEN** se inspecciona el repositorio del proyecto tras ejecutar comandos
- **THEN** no aparece ningún fichero nuevo, versionado o sin versionar

#### Scenario: Caché ilegible
- **WHEN** el fichero de caché está corrupto o no se puede leer
- **THEN** la validación se ejecuta como si no hubiera caché

### Requirement: Diagnóstico concurrente y determinista

Las comprobaciones de `hm doctor` SHALL ejecutarse concurrentemente sin alterar el orden ni
el contenido del informe.

#### Scenario: Mismo informe que en secuencial
- **WHEN** se ejecuta el diagnóstico
- **THEN** las comprobaciones aparecen en el mismo orden que al ejecutarlas una tras otra

#### Scenario: Una comprobación lenta no retrasa a las demás
- **WHEN** una comprobación tarda más que el resto
- **THEN** el tiempo total se aproxima al de la más lenta, no a la suma de todas

#### Scenario: El límite de tiempo sigue protegiendo
- **WHEN** una comprobación se cuelga
- **THEN** se reporta como aviso y el resto del informe se entrega igualmente
