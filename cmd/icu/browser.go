package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

func openBrowserURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("empty url")
	}

	var name string

	var args []string

	switch runtime.GOOS {
	case "windows":
		name = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", rawURL}
	case "darwin":
		name = "open"
		args = []string{rawURL}
	default:
		name = "xdg-open"
		args = []string{rawURL}
	}

	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}

	go func() { _ = cmd.Wait() }()

	return nil
}
