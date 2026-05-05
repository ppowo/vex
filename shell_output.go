package main

import "fmt"

var forcedShellEnv = []struct {
	name  string
	value string
}{
	{name: "PI_TASKS", value: "off"},
	{name: "PI_HASHLINE_GREP_MAX_LINES", value: "150"},
	{name: "PI_HASHLINE_GREP_MAX_BYTES", value: "10000"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD", value: "1"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_LINES", value: "400"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_BYTES", value: "25000"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_HEAD_LINES", value: "60"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_TAIL_LINES", value: "150"},
}

func emitForcedShellEnv() {
	for _, env := range forcedShellEnv {
		fmt.Printf("export %s=%q\n", env.name, env.value)
	}
}
