//go:build !windows

package main

import (
	"os"

	"golang.org/x/term"
)

func enableColorOutput() bool {
	return os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
}
