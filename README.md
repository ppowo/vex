<!-- NOTE: Keep this README minimal. -->

# vex

CLI tool to manage shell environment variables without re-sourcing.

## Setup

```sh
go generate ./...   # install mage
mage install        # build + install to ~/.bio/bin
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
```

## Details

See [ARCHITECTURE.md](ARCHITECTURE.md).
