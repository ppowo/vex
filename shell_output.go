package main

import "fmt"

var forcedShellEnv = []struct {
	name  string
	value string
}{
	{name: "PI_TASKS", value: "off"},
	{name: "PI_HASHLINE_GREP_MAX_LINES", value: "300"},
	{name: "PI_HASHLINE_GREP_MAX_BYTES", value: "20000"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD", value: "1"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_LINES", value: "600"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_MAX_BYTES", value: "40000"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_HEAD_LINES", value: "120"},
	{name: "PI_HASHLINE_BASH_CONTEXT_GUARD_TAIL_LINES", value: "180"},
}

func emitForcedShellEnv() {
	for _, env := range forcedShellEnv {
		fmt.Printf("export %s=%q\n", env.name, env.value)
	}
}
