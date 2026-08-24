package main

import (
	"fmt"
	"os"

	"github.com/cliarc/cliarc/apps/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "cliarc: %v\n", err)
		os.Exit(1)
	}
}
