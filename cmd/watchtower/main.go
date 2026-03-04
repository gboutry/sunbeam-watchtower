package main

import (
	"fmt"
	"os"

	"github.com/gboutry/sunbeam-watchtower/internal/cli"
)

func main() {
	opts := &cli.Options{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	if err := cli.NewRootCmd(opts).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
