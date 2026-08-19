## ADDED Requirements

### Requirement: Detección del contexto de worktree

La CLI SHALL determinar, en cada ejecución, si el directorio actual es un git worktree de un
proyecto Dockergento y cuál es su checkout principal.

#### Scenario: Ejecución desde el checkout principal
- **WHEN** se ejecuta cualquier comando desde el checkout principal del proyecto
- **THEN** la CLI se comporta exactamente igual que antes de este cambio

#### Scenario: Ejecución desde un worktree
- **WHEN** se ejecuta un comando desde un git worktree del proyecto
- **THEN** la CLI identifica la ruta del checkout principal

#### Scenario: Directorio fuera de git
- **WHEN** se ejecuta un comando en un directorio que no está bajo control de git
- **THEN** la CLI no lo trata como worktree y no altera su comportamiento

#### Scenario: Ruta de proyecto forzada
- **WHEN** la variable `HM_PROJECT_DIR` está definida
- **THEN** se usa su valor como checkout principal y se omite la detección automática

### Requirement: Resolución de la configuración contra el checkout principal

Desde un worktree, la CLI SHALL resolver los ficheros de Compose y las propiedades del
proyecto en el checkout principal, y SHALL invocar a Compose con ese directorio como
directorio de proyecto.

#### Scenario: Comando de ejecución desde un worktree
- **WHEN** se ejecuta `hm bash`, `hm exec` o `hm magento` desde un worktree
- **THEN** el comando se ejecuta en los contenedores del entorno del checkout principal

#### Scenario: Consulta a la base de datos desde un worktree
- **WHEN** se ejecuta `hm mysql -q` desde un worktree
- **THEN** la consulta se dirige a la base de datos del entorno del checkout principal

#### Scenario: Los montajes no se alteran
- **WHEN** se ejecuta cualquier comando permitido desde un worktree
- **THEN** los montajes de los contenedores siguen apuntando al checkout principal

### Requirement: Bloqueo de operaciones que alteran la topología

La CLI SHALL impedir, desde un worktree, la ejecución de los comandos que crean, recrean o
destruyen el entorno.

#### Scenario: Intento de arrancar desde un worktree
- **WHEN** se ejecuta `hm start` desde un worktree
- **THEN** la operación no se ejecuta
- **AND** el código de salida indica que la operación fue bloqueada por seguridad

#### Scenario: Intento de destruir el entorno desde un worktree
- **WHEN** se ejecuta `hm down -v` desde un worktree
- **THEN** la operación no se ejecuta y no se elimina ningún volumen

#### Scenario: Comandos permitidos
- **WHEN** se ejecuta desde un worktree un comando que no altera la topología, como `hm describe`, `hm logs` o `hm exec`
- **THEN** el comando se ejecuta con normalidad

#### Scenario: Parada global de contenedores
- **WHEN** se ejecuta `hm docker-stop-all` desde un worktree
- **THEN** la operación se bloquea igual que el resto

### Requirement: Mensaje de bloqueo explicativo

Cuando se bloquea una operación, la CLI SHALL explicar el contexto, la consecuencia evitada
y las alternativas disponibles.

#### Scenario: Contenido del mensaje
- **WHEN** se bloquea una operación por ejecutarse desde un worktree
- **THEN** el mensaje indica que se está en un worktree y cuál es el checkout principal
- **AND** explica que la operación habría recreado o destruido el entorno del checkout principal
- **AND** ofrece ir al checkout principal o repetir con la opción de forzado

#### Scenario: Aviso sobre el código servido
- **WHEN** se ejecuta un comando permitido desde un worktree que opera sobre el código
- **THEN** se advierte de que los contenedores sirven el código del checkout principal, no el del worktree

### Requirement: Forzado consciente

La CLI SHALL permitir ejecutar una operación bloqueada cuando se pida explícitamente, y ese
permiso SHALL aplicarse sólo a esa invocación.

#### Scenario: Operación forzada
- **WHEN** se ejecuta desde un worktree un comando bloqueado añadiendo la opción de forzado
- **THEN** la operación se ejecuta

#### Scenario: El forzado no persiste
- **WHEN** se ejecuta a continuación el mismo comando sin la opción de forzado
- **THEN** vuelve a bloquearse
