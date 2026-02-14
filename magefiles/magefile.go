package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

func Build() error {
	fmt.Println("Building vex...")

	if err := sh.Run("go", "vet", "./..."); err != nil {
		return fmt.Errorf("go vet failed: %w", err)
	}

	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}

	binary := "bin/vex"
	if runtime.GOOS == "windows" {
		binary = "bin/vex.exe"
	}

	env := map[string]string{}
	if os.Getenv("CGO_ENABLED") == "" {
		env["CGO_ENABLED"] = "0"
	}

	return sh.RunWith(env, "go", "build",
		"-ldflags=-s -w",
		"-trimpath",
		"-buildvcs=false",
		"-o", binary,
		".")
}

func Install() error {
	fmt.Println("Installing vex...")
	mg.Deps(Build)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	installDir := filepath.Join(home, ".bio", "bin")
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		return fmt.Errorf("install directory %s does not exist", installDir)
	}

	binary := "vex"
	if runtime.GOOS == "windows" {
		binary = "vex.exe"
	}

	src := filepath.Join("bin", binary)
	dst := filepath.Join(installDir, binary)

	if err := sh.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(dst, 0755); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Installed to %s\n", dst)
	return nil
}

func Clean() error {
	fmt.Println("Cleaning...")
	return sh.Rm("bin")
}

func Vet() error {
	fmt.Println("Running go vet...")
	return sh.Run("go", "vet", "./...")
}
