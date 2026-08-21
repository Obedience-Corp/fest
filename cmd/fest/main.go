package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/commands"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func main() {
	if err := commands.Execute(); err != nil {
		if code := exitCode(err); code == 0 {
			return
		}
		if !errors.Is(err, festerrors.ErrAlreadyPrinted) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

// exitCode maps a command error to a process status. A user stop (SIGINT
// wired to context cancel) is exit 0, not signal death and not "Error: ...".
func exitCode(err error) int {
	if err == nil || errors.Is(err, context.Canceled) {
		return 0
	}
	return 1
}
