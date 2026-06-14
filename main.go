package main

import (
	"os"

	"github.com/Liyonglin20030201/task061402/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
