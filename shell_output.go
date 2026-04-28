package main

import "fmt"

var forcedShellEnv = []struct {
	name  string
	value string
}{
	{name: "PI_TASKS", value: "off"},
	{name: "PI_HASHLINE_GREP_MAX_LINES", value: "300"},
	{name: "PI_HASHLINE_GREP_MAX_BYTES", value: "20000"},
}

func emitForcedShellEnv() {
	for _, env := range forcedShellEnv {
		fmt.Printf("export %s=%q\n", env.name, env.value)
	}
}
