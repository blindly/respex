package main

import (
	"os"

	"respex/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}