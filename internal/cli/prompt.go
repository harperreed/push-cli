// ABOUTME: Interactive prompting utilities for CLI input.
// ABOUTME: Handles text and secure password input from terminal.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// prompter handles basic interactive input.
type prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func newPrompter(out io.Writer) *prompter {
	if out == nil {
		out = os.Stdout
	}
	return &prompter{reader: bufio.NewReader(os.Stdin), out: out}
}

func (p *prompter) Ask(label string, defaultValue string) (string, error) {
	if _, err := fmt.Fprintf(p.out, "%s", label); err != nil {
		return "", err
	}
	if defaultValue != "" {
		if _, err := fmt.Fprintf(p.out, " [%s]", defaultValue); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprint(p.out, ": "); err != nil {
		return "", err
	}

	text, err := p.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue, nil
	}
	return text, nil
}

func (p *prompter) AskSecret(label string) (string, error) {
	if _, err := fmt.Fprintf(p.out, "%s: ", label); err != nil {
		return "", err
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytes, err := term.ReadPassword(fd)
		_, _ = fmt.Fprintln(p.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytes)), nil
	}

	text, err := p.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}
