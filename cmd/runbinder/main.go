package main

import (
	"fmt"
	"os"

	"github.com/evanjhopkins/RunBinder/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "[RUNBINDER]", err)
		os.Exit(1)
	}
}
