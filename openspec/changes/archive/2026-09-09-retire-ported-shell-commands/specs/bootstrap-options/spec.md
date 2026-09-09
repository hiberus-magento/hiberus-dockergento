# Delta for bootstrap-options

## ADDED Requirements

### Requirement: The install steps run in order through one door

`setup`'s install steps SHALL run in the order they are declared, through one runner; a step
whose command is proved in Go SHALL be answered by the Go implementation, and every other step
SHALL still reach the shell bridge.

#### Scenario: The declared order is kept

- **GIVEN** setup's declared list of install steps
- **WHEN** setup runs
- **THEN** the steps run in that exact order, unchanged by this runner

Pinned by `TestTheInstallStepsRunInOneOrderThroughOneDoor` in `dockergento/app/setup_test.go`, with
and without a dump, and by the order assertions in `tests/integration/go_setup_test.sh`.

#### Scenario: A ported step is answered natively

- **WHEN** an install step is one of the commands proved in Go (`set-host`, among others)
- **THEN** it runs through the Go implementation, not through the shell bridge

Pinned by `TestTheInstallDoorIsTheBinary` in `dockergento/dockergento_test.go`.

#### Scenario: An unported step still reaches the bridge

- **GIVEN** an install step whose command the binary's router does not answer for — `install` and
  `ssl` have no case at all
- **WHEN** that step runs through the door
- **THEN** the binary hands it to the shell bridge, and the shell entry point receives it with the
  same arguments it received before

Pinned by "a step the binary does not route reaches the bridge" in
`tests/integration/go_setup_test.sh`: the binary is asked for `install` against a recorded shell
tree, and the recorded line is exactly `--no-json install` — the tool's own global flags reach the
bridge too, because `run.go` hands `Shell()` the arguments untouched and `bin/run` splits its own
globals off itself. It is asserted with `install` and not with
`composer install`, because `composer` does have a router case and reaches the bridge only on
macOS, so a `composer` assertion would prove nothing on Linux.
