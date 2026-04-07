//go:generate go run install_tools.go

package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "init":
		cmdInit()

	case "set":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: vex set <alias> <value>\n")
			os.Exit(1)
		}
		cmdSet(args[1], args[2])

	case "unset":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: vex unset <alias>\n")
			os.Exit(1)
		}
		cmdUnset(args[1])

	case "list":
		cmdList()

	case "aliases":
		cmdAliases()

	case "path":
		cmdPath()

	case "bin":
		cmdBin(args[1:])

	case "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vex - Shell environment variable manager & binary toolkit

Usage:
  vex init                  Shell integration (eval in .zshrc or .bashrc)
  vex set <alias> <value>   Set an environment variable
  vex unset <alias>         Unset an environment variable
  vex list                  Show all variables and current values
  vex aliases               Show alias → variable mappings
  vex path                  Print the vex bin directory path
  vex bin <subcommand>      Manage curated standalone binaries

Shell Setup:
  eval "$(vex init)"`)
}
