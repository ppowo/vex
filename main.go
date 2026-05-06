//go:generate go run install_tools.go

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var cli CLI
	parser := kong.Must(&cli,
		kong.Vars{
			"version": fmt.Sprintf("vex %s (commit: %s, built: %s)", version, commit, date),
		},
		kong.Name("vex"),
		kong.Description("Shell environment variable manager & binary toolkit."),
		kong.UsageOnError(),
	)

	args := os.Args[1:]
	switch {
	case len(args) == 0:
		args = []string{"--help"}
	case len(args) == 1 && args[0] == "bin":
		args = []string{"bin", "--help"}
	}

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	err = ctx.Run()
	ctx.FatalIfErrorf(err)
}
