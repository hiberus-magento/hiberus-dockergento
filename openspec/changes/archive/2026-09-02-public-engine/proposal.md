# The tool as a library: a public engine with a facade

## Why

The separation between the engine and the command line already existed in shape: `core` and `app`
are 2,275 lines that import nothing but themselves — not Docker, not Compose, not the terminal.
What did not exist was the ability to use it.

Everything lived under `internal/`, and that is a rule of the language rather than a convention:
no other module can import it. Checked with the compiler, not assumed —

```
main.go:6:2: use of internal package .../internal/app not allowed
```

The web interface planned for later would have had nothing to talk to, and the architecture
document quietly assumed it would live inside this binary.

## What Changes

- **The engine is public**: `dockergento/` holds the domain, the ports, the use cases and the
  adapters. The adapters go with it — a consumer that cannot build the Docker adapter cannot do
  anything.
- **A facade**, which is the part that was actually missing. Describing a project means assembling
  six adapters; leaving every consumer to do that by hand guarantees each does it differently.
  `dockergento.New()` returns an engine with sensible defaults and answers the questions the tool
  answers: `Resolve`, `Describe`, `Environments`, `Diagnose`, `Start`, `Stop`, `Restart`, `Logs`,
  `Exec`.
- **The command line is a consumer like any other.** It now imports the facade and the domain
  types and nothing else — no adapters, no wiring. That is the evidence the design holds, not the
  claim.
- **`internal/cli` stays private**, and a test asserts it: the terminal is a way in, not part of
  the API.
- **A test compiles a separate module against the engine** and asks it the three questions a web
  interface asks on its first screen — what is this project, what is running on this machine, what
  is wrong — then brings an environment up through it.

Nothing about what the tool does changes. The parity tests against the shell implementation are
the net.
