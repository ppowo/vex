package main

import "fmt"

const (
	forcedEnvVarName  = "PI_TASKS"
	forcedEnvVarValue = "off"
)

func emitForcedShellEnv() {
	fmt.Printf("export %s=%q\n", forcedEnvVarName, forcedEnvVarValue)
}
