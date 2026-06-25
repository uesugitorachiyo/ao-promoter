package main

import (
	"os"

	"github.com/uesugitorachiyo/ao-promoter/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
