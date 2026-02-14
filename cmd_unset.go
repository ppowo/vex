package main

import (
	"fmt"
	"os"
)

func cmdUnset(alias string) {
	envVar, ok := aliases[alias]
	if !ok {
		fmt.Fprintf(os.Stderr, "[vex] Unknown alias: %s\n", alias)
		fmt.Fprintf(os.Stderr, "[vex] Run 'vex aliases' to see available aliases.\n")
		os.Exit(1)
	}

	state, err := readState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vex] Error reading state: %v\n", err)
		os.Exit(1)
	}

	delete(state, alias)
	if err := writeState(state); err != nil {
		fmt.Fprintf(os.Stderr, "[vex] Error saving state: %v\n", err)
		os.Exit(1)
	}

	// stdout: shell command for eval
	fmt.Printf("unset %s\n", envVar)

	// stderr: confirmation
	fmt.Fprintf(os.Stderr, "[vex] Unset %s (%s)\n", alias, envVar)
}
