package main

import (
	"fmt"
	"os"
)

// PrintHelp prints a minimal usage message.
func PrintHelp() {
	fmt.Print(
		`Peony: a calm holding space for unfinished thoughts

Usage:
  Peony <command>

Commands:
  help, -h, --help         Show this help
  version, -v, --version   Show version
  add, -a, -add			   

Planned (v0.1):
  add, tend, view, rest, release, archive`)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		PrintHelp()
		return
	}

	//CLI flag switch calls
	switch args[0] {
	case "help", "-h", "--help":
		PrintHelp()

	case "version", "-v", "--version":
		fmt.Println("Peony v0.1")
	
	
	default:
		fmt.Printf("Unknown Command: %s \n", args[0])
		PrintHelp()
		os.Exit(2)
	}
}
