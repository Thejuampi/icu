package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func promptPasswordFlagOrInteractive(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fmt.Fprint(osStdout(), "Password: ")

	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	fmt.Fprintln(osStdout())

	return string(raw), nil
}
