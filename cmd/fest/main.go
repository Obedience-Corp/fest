package main

import (
	"errors"
	"fmt"
	"os"

	// bginit must initialize before the bubbletea subtree under
	// internal/commands; its path keeps it first under gofmt.
	_ "github.com/Obedience-Corp/fest/internal/bginit"
	"github.com/Obedience-Corp/fest/internal/commands"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func main() {
	if err := commands.Execute(); err != nil {
		if !errors.Is(err, festerrors.ErrAlreadyPrinted) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}
