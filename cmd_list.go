package main

import (
	"fmt"
	"os"
	"sort"
)

func cmdList() {
	state, err := readState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vex] Error reading state: %v\n", err)
		os.Exit(1)
	}

	// Collect and sort aliases
	var keys []string
	for alias := range aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)

	for _, alias := range keys {
		envVar := aliases[alias]
		if value, ok := state[alias]; ok {
			fmt.Printf("%-10s %-30s = %s\n", alias, envVar, value)
		} else {
			fmt.Printf("%-10s %-30s   (not set)\n", alias, envVar)
		}
	}
}
