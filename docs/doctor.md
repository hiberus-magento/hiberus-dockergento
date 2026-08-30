# doctor

Runs a battery of checks over the machine and the current project, and reports for each
one what it found and **what to do about it**.

```bash
hm doctor
hm doctor --json
hm doctor --only=ports
```

It never repairs anything: diagnosing and proposing the fix is the whole job. It also
never asks for administrator privileges.

## Scopes

Checks are either **global** (they describe the machine) or **project** (they describe the
environment in the current directory). Outside a project only the global ones run, so
`hm doctor` is a valid answer to "I have just installed this, is everything in place?".

## Checks

| Id | Scope | What it looks at |
|---|---|---|
| `docker-daemon` | global | The Docker daemon is reachable |
| `compose-version` | global | Docker Compose is present and recent enough |
| `ports` | global | The ports the stack publishes, and **which project or process is holding them** |
| `disk-usage` | global | Docker volumes and dangling images piling up |
| `certificates` | global | `mkcert` and its local authority |
| `platform` | global | Docker group on Linux, free disk space on macOS |
| `compose-config` | project | The compose configuration parses |
| `properties` | project | `config/docker/properties.json` is complete and the Magento directory exists |
| `services` | project | How many services are running |
| `certificate` | project | The certificate for the domain exists and is not about to expire |
| `hosts` | project | The domain resolves locally |
| `magento` | project | `composer.lock` and whether Magento is installed |

## Severities and exit code

| Severity | Meaning | Affects the exit code |
|---|---|---|
| ok | Nothing to do | no |
| warning | Worth fixing, does not block work | no |
| error | Blocks working | **yes** |

Only errors make the command exit non-zero, which is what makes this valid:

```bash
hm doctor && hm start
```

## The port check

This is the one that answers the most common support question. When a second project
cannot start, the reason is almost always another environment already listening:

```
FAIL  ports    Ports 80, 443, 3306 are taken by the 'other-project' environment
               → cd into that project and run 'hm stop'
```

The port list comes from the compose configuration, never from a list hardcoded in the
check. Outside a project, ports held by running environments are reported as information,
not as a problem: that is the normal state of a working machine.

## The memory check

On macOS the containers do not run on your laptop. They run in a virtual machine — Colima, Docker
Desktop — with whatever memory somebody gave it once, usually when they installed it and never
since.

A machine with 48 GB whose Docker VM has 6 fits about six environments at a time. Nothing said so,
and the symptom was environments that would not start on a laptop with most of its memory free.

```
⚠ Docker has 6 GB of this machine's 48 GB, so about 8 environments fit at once
  → colima stop && colima start --memory 16 --cpu 4
```

The figure of "about N environments" comes from a measurement: an environment on the `agent`
profile — php, nginx, database, search and Redis — costs around 550 MB, and the search engine is
85% of that. A full stack costs more.

It warns when the VM has less than a quarter of a machine that has memory to spare, and fails when
it has under 4 GB, which is not enough for one project. On Linux both numbers are the same and it
always passes, which is the correct answer there.

## Robustness

Each check runs in its own process with a time limit. A check that hangs or crashes is
reported as a warning and **the rest of the diagnosis carries on** — a doctor that dies
halfway is worse than no doctor at all.
