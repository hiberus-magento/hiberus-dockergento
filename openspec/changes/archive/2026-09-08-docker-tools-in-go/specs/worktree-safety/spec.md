# Delta for worktree-safety

## MODIFIED Requirements

### Requirement: Bloqueo de operaciones que alteran la topología

La CLI SHALL impedir, desde un worktree, la ejecución de los comandos que crean, recrean o
destruyen el entorno, sin importar cuál de sus implementaciones responde a ese comando.
(Previously: worded around the shell bridge answering every not-yet-ported command; now stated to
hold regardless of which implementation answers, since some of these commands are answered
directly by the tool.)

#### Scenario: Intento de arrancar desde un worktree
- **WHEN** se ejecuta `hm start` desde un worktree
- **THEN** la operación no se ejecuta
- **AND** el código de salida indica que la operación fue bloqueada por seguridad

#### Scenario: Intento de destruir el entorno desde un worktree
- **WHEN** se ejecuta `hm down -v` desde un worktree
- **THEN** la operación no se ejecuta y no se elimina ningún volumen

#### Scenario: Comandos permitidos
- **WHEN** se ejecuta desde un worktree un comando que no altera la topología, como `hm describe`, `hm logs`, `hm exec` o `hm docker-compose`
- **THEN** el comando se ejecuta con normalidad
- **AND** `hm docker-compose` sigue sin estar entre los que se bloquean, igual que en la implementación de shell: pasar un subcomando a Compose no es alterar la topología por sí mismo

#### Scenario: Parada global de contenedores
- **WHEN** se ejecuta `hm docker-stop-all` desde un worktree
- **THEN** la operación se bloquea igual que el resto
