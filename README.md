# vex

CLI tool to manage shell environment variables without re-sourcing, plus a curated binary toolkit.

> **Note:** macOS and Linux only. Windows is not supported.

## Setup

```sh
go generate ./...   # install mage
mage install        # build + install to ~/.bio/bin
```

Add to `.zshrc` or `.bashrc`:

```sh
eval "$(vex init)"
```

`vex init` always exports `PI_TASKS=off`, `PI_HASHLINE_GREP_MAX_LINES=300`, `PI_HASHLINE_GREP_MAX_BYTES=20000`, and the bash context guard defaults (`PI_HASHLINE_BASH_CONTEXT_GUARD=1`, `PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_LINES=600`, `PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_BYTES=40000`, `PI_HASHLINE_BASH_CONTEXT_GUARD_HEAD_LINES=120`, `PI_HASHLINE_BASH_CONTEXT_GUARD_TAIL_LINES=180`) as part of shell setup.

## Environment Variables

```sh
vex set aws staging       # exports AWS_PROFILE=staging
vex unset aws             # unsets AWS_PROFILE
vex list                  # show aliased vars, current values, and forced defaults
vex aliases               # show alias → variable mappings
```

### Aliases

Short aliases map to real environment variable names (hardcoded in `aliases.go`):

| Alias | Variable |
|-------|----------|
| exa | EXA_API_KEY |
| openrouter | OPENROUTER_API_KEY |
| opencode | OPENCODE_API_KEY |
| synthetic | SYNTHETIC_API_KEY |

State persists in `~/.vex/state.json` and is replayed on shell startup.

## Bin Directory

`vex init` creates an OS-dependent bin directory and adds it to `$PATH`:

| OS | Path |
|----|------|
| macOS | `~/.local/share/vex` |
| Linux | `$XDG_DATA_HOME/vex` (defaults to `~/.local/share/vex`) |

```sh
vex path              # print the vex bin directory
```

Place scripts or binaries here to make them available in every shell.

## Managed Binaries (`vex bin`)

Install and update curated tools into the vex bin directory:

```sh
vex bin install <tool> [--force]   # install a curated tool
vex bin ls                         # list all curated tools and their status
vex bin status <tool>              # detailed install/update status
vex bin sync [--dry-run]           # install missing + update outdated tools
vex bin update [<tool>|--all] [--force]  # update one or all managed tools
vex bin version <tool>             # show installed vs latest version
```

### Available Tools

| Tool | Binary | Description |
|------|--------|-------------|
| ast-grep | `sg` | Fast, polyglot code search and rewriting |
| cs | `cs` | Ranked structural code search |
| difftastic | `difft` | Structural diff that understands syntax |
| fd | `fd` | A simple, fast and user-friendly alternative to find |
| jbang | `jbang` | Java scripting and app launcher (requires a system JDK) |
| nushell | `nu` | A new type of shell |
| nu-plugin-semver | `nu_plugin_semver` | SemVer parsing for Nushell |
| nu-plugin-file | `nu_plugin_file` | File type detection via libmagic |
| scc | `scc` | Fast code counter with complexity |
| shellcheck | `shellcheck` | Shell script static analysis |
| universal-ctags | `ctags` | Maintained ctags implementation for source code indexing |
| yq | `yq` | YAML/JSON/XML/CSV processor |

JBang is only managed when a system JDK is already available via `JAVA_HOME` or `javac` on `PATH`. Otherwise `vex bin ls` shows it as unavailable and `vex bin sync` skips it.

Only hardcoded tools are supported — vex never assumes all files in the bin directory are managed.

## Details

See [ARCHITECTURE.md](ARCHITECTURE.md) for implementation details.
