package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("aegis dev")
		return
	}

	fmt.Println("AEGIS command-line interface")
	fmt.Println("Usage: aegis [--version]")
}
