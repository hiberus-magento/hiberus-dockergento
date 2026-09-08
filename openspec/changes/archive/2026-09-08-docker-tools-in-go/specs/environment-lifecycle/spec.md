# Delta for environment-lifecycle

## MODIFIED Requirements

### Requirement: Parar toda la máquina exige una respuesta

El comando que para todos los contenedores de la máquina SHALL pedir confirmación e informar de su
alcance.
(Previously: answered by `bin/run`'s shell implementation; answered by the tool itself now, with
the same scope, wording and exit codes, and no shell process involved.)

#### Scenario: Se dice cuántos y de quién
- **WHEN** se pide parar todos los contenedores de forma interactiva
- **THEN** se indica cuántos se van a parar y cuántos no pertenecen al proyecto actual, con el
  texto exacto "This stops N container(s) on this machine."
- **AND** se pregunta con el texto exacto "Stop them all? [y/N]: "

#### Scenario: No confirmar no para nada
- **WHEN** no se confirma
- **THEN** ningún contenedor se detiene, y se informa con el texto exacto "Nothing was stopped."

#### Scenario: Nada que parar
- **WHEN** no hay contenedores en marcha
- **THEN** se informa con el texto exacto "No containers running" y no se pregunta

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se paran sin preguntar

#### Scenario: Confirming stops them

- **WHEN** the question is answered yes
- **THEN** every running container is stopped, the count stopped matches the count announced, and
  it is reported with the exact text "Stopping N container(s)"

#### Scenario: The "does not belong" line is conditional

- **WHEN** every running container belongs to the current project
- **THEN** only the total is announced, and the line "M of them do not belong to '<project>'." is
  never printed
- **AND** when at least one does not belong, that exact line is printed with the actual count

#### Scenario: A machine-readable answer

- **WHEN** the output is not a terminal
- **THEN** the answer is the JSON document `{total, others, stopped}`, in place of the text
  messages the interactive run prints
(Deliberate change from the shell twin, which always printed text.)

#### Scenario: Some containers refuse to stop

- **WHEN** stopping fails for some of the containers
- **THEN** the rest are still stopped, the failures are reported with the Docker-failure exit
  code and name the containers that refused, and the count stopped is the count that actually
  stopped
(Deliberate change from the shell twin, which forwarded Docker's own exit status.)

#### Scenario: Starting with -s asks the same question

- **WHEN** `hm start -s` finds other containers running
- **THEN** it asks the same question `docker-stop-all` asks, with the same exact text, and asks
  nothing when non-interactive mode is set

#### Scenario: An API caller is never asked

- **WHEN** the machine-wide stop is requested by an API caller
- **THEN** no question is asked
