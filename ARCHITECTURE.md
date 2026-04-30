# Architecture

## Problem

A child process cannot modify its parent shell's environment. `vex` solves this using the **eval pattern** (same as `cld`, `rbenv`, `direnv`).

## How It Works

### `vex init`

Outputs shell code to stdout, designed to be eval'd in `.zshrc` or `.bashrc`:

```sh
eval "$(vex init)"
```

The output includes:
2. **PATH addition** — adds the vex bin directory to `$PATH` (with deduplication)
3. **Shell function wrapper** — selectively evals `set`/`unset` commands

```sh
vex() {
  if [[ "$1" == "set" || "$1" == "unset" ]]; then
    eval "$(command vex "$@")"
  else
    command vex "$@"
  fi
}
```

### `set` / `unset` Commands

Output shell statements to **stdout** (captured by eval) and info to **stderr** (displayed directly).
`vex init` always emits forced shell setup exports for `PI_TASKS="off"`, `PI_HASHLINE_GREP_MAX_LINES="300"`, `PI_HASHLINE_GREP_MAX_BYTES="20000"`, and bash context guard defaults (`PI_HASHLINE_BASH_CONTEXT_GUARD="1"`, `PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_LINES="600"`, `PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_BYTES="40000"`, `PI_HASHLINE_BASH_CONTEXT_GUARD_HEAD_LINES="120"`, `PI_HASHLINE_BASH_CONTEXT_GUARD_TAIL_LINES="180"`). These values are hardcoded and not persisted in state.

### Persistent State

`~/.vex/state.json` stores alias→value pairs:
```json
{
  "aws": "staging",
  "region": "eu-west-1"
}
```

- `vex set` writes to this file + outputs `export` to stdout
- `vex unset` removes from this file + outputs `unset` to stdout
- `vex init` reads this file + replays all exports, then emits forced default exports including `PI_TASKS=off`, grep budget defaults, and bash context guard defaults

## Variable Aliases

Short aliases mapped to real env var names, hardcoded in `aliases.go`:

```go
var aliases = map[string]string{
    "aws":    "AWS_PROFILE",
    "region": "AWS_REGION",
    "node":   "NODE_ENV",
}
```

## Bin Directory

`vex init` automatically creates an OS-dependent bin directory and adds it to `$PATH`:

| OS | Path |
|----|------|
| macOS | `~/.local/share/vex` |
| Linux | `$XDG_DATA_HOME/vex` (defaults to `~/.local/share/vex`) |
> **Note:** Windows is not supported. vex relies on zsh/bash shell integration (both are supported).

Use `vex path` to print the resolved path. Place scripts or binaries in this directory to make them available everywhere.

## Managed Binaries (`vex bin`)

vex can install and manage curated tools into its bin directory. Only tools hardcoded into vex are supported — vex never treats all files in the bin directory as managed.

### Tool Catalog

Each tool is defined as a `ToolSpec` in `internal/bin/catalog.go` with:
- Name, binary name, version detection args
- An optional availability check (for prerequisites like a system JDK)
- An optional install finalizer for generated launchers or post-processing
- A resolver function that fetches the latest release from GitHub
### State Tracking

`~/.vex/bin-state.json` tracks which tools are installed, their versions, and artifact metadata.

`jbang` is installed as a vex-generated launcher plus a sibling `jbang.jar`. vex only manages it when a system JDK is already available via `JAVA_HOME` or `javac` on `PATH`; otherwise it is reported as unavailable and skipped by `sync` / `update --all`.

### Subcommands

| Command | Description |
|---------|-------------|
| `vex bin install <tool> [--force]` | Install a curated tool |
| `vex bin ls` | List all curated tools and their status |
| `vex bin status <tool>` | Detailed install/update status |
| `vex bin sync [--dry-run]` | Install missing + update outdated tools |
| `vex bin update [<tool>\|--all] [--force]` | Update one or all managed tools |
| `vex bin version <tool>` | Show installed vs latest version |

## Commands
| Command | Eval'd | Description |
|---------|--------|--------------|
| `vex init` | Yes (once, in `.zshrc` or `.bashrc`) | Replay state + create bin dir + add to PATH + output shell function wrapper |
| `vex set <alias> <value>` | Yes | Export var + persist to state file |
| `vex unset <alias>` | Yes | Unset var + remove from state file |
| `vex list` | No | Show aliased vars, current values, and forced defaults |
| `vex aliases` | No | List alias → variable mappings |
| `vex path` | No | Print the vex bin directory path |
| `vex bin <subcommand>` | No | Manage curated standalone binaries |

## Output Streams

- **stdout** — Shell commands (`export`, `unset`). `vex init` setup output also includes forced default exports for `PI_TASKS` and pi-hashline grep budgets.
- **stderr** — Human-readable messages, errors, confirmations.

## Project Structure

```
vex/
├── main.go              # Entrypoint, command dispatch
├── aliases.go           # Hardcoded alias → env var mappings
├── paths.go             # OS-dependent bin directory resolution
├── state.go             # Read/write ~/.vex/state.json
├── cmd_init.go          # Shell function + state replay + bin dir setup
├── cmd_set.go           # Set variable
├── cmd_unset.go         # Unset variable
├── cmd_list.go          # List aliased vars and forced shell defaults
├── cmd_aliases.go       # List alias mappings
├── cmd_path.go          # Print bin directory path
├── cmd_bin.go           # CLI layer for bin subcommands
├── internal/
│   ├── paths/
│   │   └── paths.go     # Managed bin dir + config dir resolution
│   └── bin/
│       ├── catalog.go   # ToolSpec type + hardcoded tool catalog
│       ├── tool.go      # Install/Update/Inspect engine, download, extract
│       ├── state.go     # bin-state.json read/write
│       ├── cs.go        # cs resolver + shared GitHub types/helpers
│       ├── ctags.go     # universal-ctags nightly resolver
│       ├── ast_grep.go  # ast-grep resolver
│       ├── difftastic.go
│       ├── fd.go        # fd resolver
│       ├── nushell.go
│       ├── nushell_plugins.go
│       ├── scc.go
│       ├── shellcheck.go
│       └── yq.go
├── install_tools.go     # go:generate installs mage
├── magefiles/
│   └── magefile.go      # Build, install, clean, vet targets
├── go.mod
├── ARCHITECTURE.md
└── README.md
```

## Build

Uses [mage](https://magefile.org/). Installs to `~/.bio/bin`.

```sh
go generate ./...   # Install mage (first time)
mage build          # Build to bin/vex
mage install        # Build + copy to ~/.bio/bin
mage clean          # Remove bin/
mage vet            # Run go vet
```
