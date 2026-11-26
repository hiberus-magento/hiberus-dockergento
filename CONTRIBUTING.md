# Contributing to hm

Thanks for helping. `hm` is a Go binary; everything below needs only Go (the version in `go.mod`)
and, for the shell suites, Bash.

## Quick path

```bash
git clone https://github.com/hiberus-magento/hiberus-dockergento.git
cd hiberus-dockergento
go build -o bin/hm ./cmd/hm                              # the binary (bin/hm is not versioned)
DOCKER_HOST=unix:///nonexistent go test ./...            # the Go tests
DOCKER_HOST=unix:///nonexistent ./tests/run.sh unit      # the fast shell suites
gofmt -l ./cmd ./internal ./dockergento ./test && go vet ./...
```

`gofmt -l` must print nothing. These are the checks CI runs on every push to `main` or a
`release/**` branch and on every pull request, on Ubuntu and macOS
([`.github/workflows/go.yml`](.github/workflows/go.yml)); CI runs `go test` with `-race`.

## Rules that save you a broken machine

- **Never let a test reach your Docker daemon.** `test/e2e` builds the binary and uses whatever
  Docker it finds, creating and removing containers in it. Set `DOCKER_HOST=unix:///nonexistent`
  for every broad run, `go test ./...` included.
- Run `./tests/run.sh unit`, never `./tests/run.sh tests/unit`: the second one runs every suite,
  including the ones that need Docker. One suite on its own:
  `./tests/run.sh tests/integration/<suite>_test.sh`.
- Tests go through the public interface: handlers are tested through `Run` with the fake engine
  (`internal/cli/fake_engine_test.go`), and stable output is pinned with goldens under `testdata/`.

## Commits and pull requests

- English, [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
  `docs:`, `refactor:`, `test:`, `chore:` ...), one reviewable unit per commit with its tests and
  docs.
- No AI attribution in commits: no `Co-Authored-By` trailers for tools.
- A new or changed command, option or message updates its help (`data/command_descriptions.json`)
  and its page under [`docs/`](docs/README.md).

## Where things are

| You want to | Start at |
|---|---|
| Understand how the tool is built | [ARCHITECTURE.md](ARCHITECTURE.md) and `architecture/` |
| Add a command or a container operation | "Key Files for Common Tasks" in [CLAUDE.md](CLAUDE.md) |
| Change service versions or default properties | `data/requirements.json`, `data/properties.json` |
| Change the Compose templates | `docker-compose/` |
| Know what the project has decided | `.specify/memory/constitution.md`, [ROADMAP.md](ROADMAP.md) (in Spanish) |

## Working on hm itself

The binary is self-contained: `data/`, `docker-compose/` and `skills/` are compiled into it with
`go:embed`, so it reads nothing from beside the executable and an edit to one of those files
needs a rebuild (`go build -o bin/hm ./cmd/hm`) to be seen. A new data file must also be added to
the `//go:embed` list in `assets.go`.

To try an edit without rebuilding, point `HM_ASSETS_DIR` at a checkout:

```bash
HM_ASSETS_DIR=~/src/hiberus-dockergento hm compatibility
```

When it is set, every one of those files is read from that directory (laid out like the root of
the repository) instead of from the binary, with no fallback to the embedded copy: a file missing
there is an error. Project scripts under `config/hm/commands` get `DATA_DIR` set to
`$HM_ASSETS_DIR/data` then (see [project commands](docs/project-commands.md#what-the-script-gets)
for the case without it).

## Branches and releases

| Branch or tag | Meaning |
|---|---|
| `main` | Stable. Reached through a pull request, tagged `X.Y.Z` |
| `feature/*` | Work in progress |
| `release/X.Y.Z` | Freeze and validate |
| `X.Y.Z-rc.N` | Release candidates, shared for internal validation |

Tags carry no `v`. A tag builds the release binaries with GoReleaser (`.goreleaser.yaml`,
[`.github/workflows/release.yml`](.github/workflows/release.yml)); the installer's tests are
`tests/unit/installer_test.sh`. Pushing, opening pull requests and releasing are maintainer
decisions.
