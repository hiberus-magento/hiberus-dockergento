# environment-lifecycle Specification

## Purpose
TBD - created by archiving change protect-environment-lifecycle. Update Purpose after archive.
## Requirements
### Requirement: Destruir volúmenes exige una respuesta

`hm down` con la opción de borrar volúmenes SHALL pedir confirmación antes de destruir nada.

#### Scenario: Se enumera lo que se va a perder
- **WHEN** se pide destruir el entorno con sus volúmenes de forma interactiva
- **THEN** se nombran los volúmenes que se van a borrar antes de preguntar

#### Scenario: No confirmar no destruye
- **WHEN** no se confirma
- **THEN** los volúmenes siguen existiendo y el entorno queda como estaba

#### Scenario: Se ofrece guardar antes
- **WHEN** se pide destruir el entorno con sus volúmenes
- **THEN** se ofrece guardar una copia de la base de datos como primera opción

#### Scenario: Guardar y destruir
- **WHEN** se acepta guardar
- **THEN** queda una copia restaurable y sólo después se destruye el entorno

#### Scenario: Destruir sin guardar
- **WHEN** se elige destruir sin guardar
- **THEN** no se crea ninguna copia y el entorno se destruye

#### Scenario: Sin volúmenes no se pregunta
- **WHEN** se destruye el entorno sin pedir borrar volúmenes
- **THEN** no se pregunta nada, porque no se pierde nada

#### Scenario: Sin nadie a quien preguntar
- **WHEN** no hay terminal, o se ha indicado que no se pregunte
- **THEN** se destruye sin preguntar, como hasta ahora

### Requirement: Guardar al parar

`hm stop` SHALL poder guardar una copia de la base de datos antes de parar el entorno.

#### Scenario: Se pide la copia
- **WHEN** se para el entorno indicando que se quiere una copia
- **THEN** la copia queda guardada antes de parar los servicios

#### Scenario: Parar no guarda por su cuenta
- **WHEN** se para el entorno sin indicar nada
- **THEN** no se crea ninguna copia y el comando se comporta como siempre

#### Scenario: Si la copia falla no se para
- **WHEN** la copia no se puede crear
- **THEN** el entorno no se para, para no dejar creer que hay una copia que no existe

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

### Requirement: Deleting the data is asked about

The tool SHALL NOT delete an environment's volumes without saying what it is about to delete and
offering to save a copy first.

#### Scenario: Removing an environment with its volumes

- **WHEN** the environment is removed with its volumes, interactively, and there are volumes to
  delete
- **THEN** they are named, and the answers offered are to save a copy first, to destroy without
  saving, or to do nothing

#### Scenario: The copy cannot be taken

- **WHEN** saving a copy first is chosen and the copy fails
- **THEN** nothing is destroyed, and the command says so

#### Scenario: Nothing to lose

- **WHEN** the environment has no volumes left to delete
- **THEN** nothing is asked

#### Scenario: Not interactive

- **WHEN** the run is not interactive
- **THEN** the volumes are deleted without a question, because the flag was explicit

### Requirement: Removing an environment is Compose's own command

The tool SHALL do what Compose does, and SHALL NOT remove more than it was asked to.

#### Scenario: Containers of a project no longer in the file

- **WHEN** an environment is removed without asking for orphans to go
- **THEN** they are left alone

#### Scenario: A branch environment being erased

- **WHEN** a branch environment is removed
- **THEN** its orphans go with it, because the environment is being erased rather than stopped

#### Scenario: An option that takes a value refuses when none follows

- **GIVEN** `hm down` is run with `--rmi` or with `-t`/`--timeout` as the last argument, with
  nothing after it
- **WHEN** the parse loop reaches that flag
- **THEN** the command exits 2, the environment is not touched, and in text mode stderr carries
  the legible message naming the flag plus the `  → ` hint while stdout is empty
- **AND** in `--json` mode stderr carries exactly one JSON envelope with `ok: false`,
  `error.code` 2, `error.type` "missing_value", a message and a hint, and stdout is still empty

Test: `TestDownRefusesAMissingRmiValue`, `TestDownRefusesAMissingTimeoutValue` (new, in
`internal/cli/down_test.go`; each with a text and a `--json` subtest, called directly against
`down(...)` with plain buffers — the refusal returns before `projectOr` and before the engine
call, so no fake and no Docker is involved).

#### Scenario: A missing value and a bad value stay distinguishable

- **GIVEN** `-t`/`--timeout` is the flag being refused
- **WHEN** the value is missing versus when a value is present but not a number (e.g.
  `hm down -t nada`)
- **THEN** the missing case answers `error.type` "missing_value" and the non-numeric case
  answers `error.type` "invalid_argument" — the two refusals are never collapsed into one type

Test: `TestDownRefusesAMissingTimeoutValue`, `TestDownStillRefusesANonNumericTimeout` (new; the
second pins the existing `invalid_argument` behavior so it cannot regress alongside the new
guard).

#### Scenario: A value that is given is still accepted

- **GIVEN** `--rmi all` or `-t 30`, alone or combined with `-v` and `--remove-orphans`
- **WHEN** `down` parses those arguments
- **THEN** the parse loop runs to the end without refusing, and the command reaches project
  resolution — from a directory that is not a project, that is exit 4 with `error.type`
  "project_not_configured", which is the exact answer asserted, never "some code other than 2"
- **AND** an option nobody declared still fails exactly as before, with the existing
  `invalid_argument` "Unknown option" usage error and exit 2

Test: `TestDownStillAcceptsAGivenValue`, `TestADownOptionNobodyDeclaredIsAUsageError` (new; both
call `down(...)` directly with plain buffers from a temporary working directory, so resolution
fails predictably at `engine.go:84-89` and the run stops before `down.go:61` ever reaches a real
engine).

**What this scenario does not prove, deliberately.** That `--rmi all` still *stores* `"all"` in
`core.DownOptions.Images`, and `-t 30` still stores `30`, is not observable from outside `down()`
while `Down` is off the `commands` interface. An assertion phrased as "did not refuse" would pass
against a fix that adds the guard and forgets the assignment — which is the original defect wearing
a different hat. Asserting the exact resolution failure is the strongest statement available
without the seam; proving the value reaches the engine is owed to `down-go-tests`, whose
`TestWhatDownAsks` reads it back off the fake's recorded `DownOptions`. This is a stated gap, not
an oversight.
