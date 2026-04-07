# Architecture

## Problem

A child process cannot modify its parent shell's environment. `vex` solves this using the **eval pattern** (same as `cld`, `rbenv`, `direnv`).

## How It Works

### `vex init`

Outputs shell code to stdout, designed to be eval'd in `.zshrc`:

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
- `vex init` reads this file + replays all exports

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
| macOS | `~/.local/share/vex/bin` |
| Linux | `$XDG_DATA_HOME/vex/bin` (defaults to `~/.local/share/vex/bin`) |
> **Note:** Windows is not supported. vex relies on zsh/bash shell integration.

Use `vex path` to print the resolved path. Place scripts or binaries in this directory to make them available everywhere.

## Commands
| Command | Eval'd | Description |
|---------|--------|--------------|
| `vex init` | Yes (once, in `.zshrc`) | Replay state + create bin dir + add to PATH + output shell function wrapper |
| `vex set <alias> <value>` | Yes | Export var + persist to state file |
| `vex unset <alias>` | Yes | Unset var + remove from state file |
| `vex list` | No | Show current values of all aliased variables |
| `vex aliases` | No | List alias → variable mappings |
| `vex path` | No | Print the vex bin directory path |

## Output Streams

- **stdout** — Shell commands (`export`, `unset`). Only consumed by eval.
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
├── cmd_list.go          # List current values
├── cmd_aliases.go       # List alias mappings
├── cmd_path.go          # Print bin directory path
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
