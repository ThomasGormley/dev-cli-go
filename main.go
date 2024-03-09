package main

import (
	"fmt"
	"os"

	cli "github.com/thomasgormley/dev-cli-go/internal"
)

func main() {
	if err := cli.Run(os.Args, os.Stdin, os.Stdout, os.Stderr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
