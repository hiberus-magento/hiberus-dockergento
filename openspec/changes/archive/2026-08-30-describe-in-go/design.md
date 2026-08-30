# Design

## Reading Compose, not running it

`compose-go` is what Compose uses to turn files into a project: merging several of them,
interpolating variables, applying `!reset`, resolving profiles. Using it means the tool reads
exactly what `docker compose` would read, without a subprocess.

Running is a different question and is answered in ADR-009: bringing environments up stays with
the `docker compose` command, because embedding the engine is 395 modules and an 84 MB binary,
and reimplementing it means reimplementing the labels and the configuration hash that decide
whether a container belongs to a project.

The test does not check our conversion against itself. It runs `docker compose config` on the same
files and compares — name, services, images, published ports, environment. A library that drifts
from the command would change the meaning of every read path at once, quietly.

## Six questions at once

The description is built from six things that have nothing to do with each other: the compose
configuration, the containers, the project's own files, the tool's version, Compose's version, and
whether Xdebug is loaded. Three of those are subprocesses.

In a shell they can only happen in sequence. Here they are started together and waited for, which
is most of the difference between 285 ms and 94 ms — and it is the plainest example of what the
migration is for. Nothing clever: seven goroutines and a wait.

## What the comparison caught

The properties reader read the project's file and stopped. The shell implementation merges the
tool's `data/properties.json` underneath, which is where `WORKDIR_PHP`, the compose file names and
the mail catcher come from for every project that never set them.

The Go version therefore answered with empty strings for all of them — and the JSON looked
perfectly well-formed. It was found because the two implementations were compared field by field
on a project fixture, which is the argument for porting this way rather than by reading the shell
and rewriting it.

One consequence worth keeping: a directory with no properties of its own still resolves to *no
project*, not to the defaults. Otherwise every directory on the machine would look configured.
