package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// promptPasswordFlagOrInteractive prompts on stderr so stdout stays clean JSON.
func promptPasswordFlagOrInteractive(stderr io.Writer, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fmt.Fprint(stderr, "Password: ")

	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	fmt.Fprintln(stderr)

	return string(raw), nil
}
