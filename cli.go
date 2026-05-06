package main

import (
	"errors"

	"github.com/alecthomas/kong"
)

type CLI struct {
	Version kong.VersionFlag `short:"v" help:"Show version and exit."`
	Init    InitCmd    `cmd:"" help:"Shell integration (eval in .zshrc or .bashrc)."`
	Set     SetCmd     `cmd:"" help:"Set an environment variable."`
	Unset   UnsetCmd   `cmd:"" help:"Unset an environment variable."`
	List    ListCmd    `cmd:"" help:"Show all variables and current values."`
	Aliases AliasesCmd `cmd:"" help:"Show alias → variable mappings."`
	Path    PathCmd    `cmd:"" help:"Print the vex bin directory path."`
	Bin     BinCmd     `cmd:"" help:"Manage curated standalone binaries."`
}

type InitCmd struct{}

func (c *InitCmd) Run() error {
	cmdInit()
	return nil
}

type SetCmd struct {
	Alias string `arg:"" name:"alias" help:"Alias to set."`
	Value string `arg:"" name:"value" help:"Value to assign." passthrough:"all"`
}

func (c *SetCmd) Run() error {
	cmdSet(c.Alias, c.Value)
	return nil
}

type UnsetCmd struct {
	Alias string `arg:"" name:"alias" help:"Alias to unset."`
}

func (c *UnsetCmd) Run() error {
	cmdUnset(c.Alias)
	return nil
}

type ListCmd struct{}

func (c *ListCmd) Run() error {
	cmdList()
	return nil
}

type AliasesCmd struct{}

func (c *AliasesCmd) Run() error {
	cmdAliases()
	return nil
}

type PathCmd struct{}

func (c *PathCmd) Run() error {
	cmdPath()
	return nil
}

type BinCmd struct {
	Install BinInstallCmd `cmd:"" help:"Install a curated standalone binary."`
	Ls      BinLsCmd      `cmd:"" help:"List curated managed binaries."`
	Status  BinStatusCmd  `cmd:"" help:"Show install and update status."`
	Sync    BinSyncCmd    `cmd:"" help:"Install missing and update outdated binaries."`
	Update  BinUpdateCmd  `cmd:"" help:"Update one or all managed binaries."`
	Version BinVersionCmd `cmd:"" help:"Show installed and latest version."`
}

func (c *BinCmd) Help() string {
	return "Only binaries hardcoded into vex are supported."
}

type BinInstallCmd struct {
	Tool  string `arg:"" name:"tool" help:"Managed tool to install."`
	Force bool   `help:"Overwrite an existing installation or take over an unmanaged binary."`
}

func (c *BinInstallCmd) Run() error {
	cmdBinInstall(c.Tool, c.Force)
	return nil
}

type BinLsCmd struct{}

func (c *BinLsCmd) Run() error {
	cmdBinLs()
	return nil
}

type BinStatusCmd struct {
	Tool string `arg:"" name:"tool" help:"Managed tool to inspect."`
}

func (c *BinStatusCmd) Run() error {
	cmdBinStatus(c.Tool)
	return nil
}

type BinSyncCmd struct {
	DryRun bool `help:"Show what would change without installing or updating binaries."`
}

func (c *BinSyncCmd) Run() error {
	cmdBinSync(c.DryRun)
	return nil
}

type BinUpdateCmd struct {
	Tool  string `arg:"" name:"tool" optional:"" help:"Managed tool to update."`
	All   bool   `help:"Update all managed binaries."`
	Force bool   `help:"Reinstall even when the latest version is already installed."`
}

func (c *BinUpdateCmd) Validate() error {
	if c.All == (c.Tool == "") {
		return errors.New("specify either <tool> or --all")
	}
	return nil
}

func (c *BinUpdateCmd) Run() error {
	cmdBinUpdate(c.Tool, c.All, c.Force)
	return nil
}

type BinVersionCmd struct {
	Tool string `arg:"" name:"tool" help:"Managed tool to compare."`
}

func (c *BinVersionCmd) Run() error {
	cmdBinVersion(c.Tool)
	return nil
}
