package main

import (
	"os"

	"github.com/divijg19/peony/internal/cli"
)

func main() {
	os.Exit(cli.RunBloom(os.Args[1:]))
}
