//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	tools := []string{
		"github.com/magefile/mage@latest",
	}

	for _, tool := range tools {
		fmt.Printf("Installing %s...\n", tool)
		cmd := exec.Command("go", "install", tool)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install %s: %v\n", tool, err)
			os.Exit(1)
		}
	}
	fmt.Println("All tools installed successfully!")
}
