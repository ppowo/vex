<!-- NOTE: Keep this README minimal. -->

# vex

CLI tool to manage shell environment variables without re-sourcing.

> **Note:** macOS and Linux only. Windows is not supported.

## Setup

```sh
go generate ./...   # install mage
mage install        # build + install to ~/.local/share/vex
```

Add to `.zshrc`:

```sh
eval "$(vex init)"
```

## Usage

```sh
vex set aws staging       # exports AWS_PROFILE=staging
vex unset aws             # unsets AWS_PROFILE
vex list                  # show all aliased vars and current values
vex aliases               # show alias → variable mappings
vex path                  # print the vex bin directory
```

The vex bin directory (printed by `vex path`) is automatically created and added to `$PATH` by `vex init`. Place scripts or binaries there to make them available in every shell.

## Details

See [ARCHITECTURE.md](ARCHITECTURE.md).
