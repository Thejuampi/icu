package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

func promptPasswordFlagOrInteractive(stdout io.Writer, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fmt.Fprint(stdout, "Password: ")

	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	fmt.Fprintln(stdout)

	return string(raw), nil
}
