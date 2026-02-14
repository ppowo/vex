package main

import (
	"fmt"
	"os"
)

func cmdInit() {
	state, err := readState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vex] Warning: could not read state: %v\n", err)
		state = make(map[string]string)
	}

	// Replay persisted state as exports
	for alias, value := range state {
		envVar, ok := aliases[alias]
		if !ok {
			continue
		}
		fmt.Printf("export %s=%q\n", envVar, value)
	}

	// Output shell function wrapper
	fmt.Print(`
vex() {
  if [[ "$1" == "set" || "$1" == "unset" ]]; then
    eval "$(command vex "$@")"
  else
    command vex "$@"
  fi
}
`)
}
