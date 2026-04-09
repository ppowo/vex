package main

import (
	"fmt"
	"os"
)

func cmdInit() {
	// Ensure the vex bin directory exists
	if err := ensureBinDir(); err != nil {
		fmt.Fprintf(os.Stderr, "[vex] Warning: could not create bin directory: %v\n", err)
	}
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

	emitForcedShellEnv()

	// Add vex bin directory to PATH (with deduplication)
	binDir := vexBinDir()
	fmt.Printf(`
case ":$PATH:" in
  *":%s:"*) ;;
  *) export PATH="%s:$PATH" ;;
esac
`, binDir, binDir)
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
