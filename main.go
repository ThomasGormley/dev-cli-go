package main

import (
	"fmt"
	"os"

	cli "github.com/thomasgormley/dev-cli-go/internal"
	"github.com/thomasgormley/dev-cli-go/internal/gh"
)

func main() {
	if err := cli.Run(
		os.Args,
		os.Stdout,
		os.Stderr,
		gh.NewGitHubClient(os.Stderr, os.Stdout, os.Stdin),
		nil,
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
