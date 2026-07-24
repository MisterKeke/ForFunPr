package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type prompter struct {
	reader *bufio.Reader
	output io.Writer
}

func newPrompter(command *cobra.Command) *prompter {
	return &prompter{
		reader: bufio.NewReader(command.InOrStdin()),
		output: command.ErrOrStderr(),
	}
}

func (prompt *prompter) required(
	label string,
	value *string,
) error {
	if strings.TrimSpace(*value) != "" {
		return nil
	}

	if _, err := fmt.Fprintf(prompt.output, "%s: ", label); err != nil {
		return fmt.Errorf("write %s prompt: %w", label, err)
	}

	input, err := prompt.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read %s: %w", label, err)
	}

	*value = strings.TrimSpace(input)
	if *value == "" {
		return fmt.Errorf("%s is required", label)
	}

	return nil
}

func parseInteger(label string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", label, err)
	}
	return parsed, nil
}

func parseFloat(label string, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", label, err)
	}
	return parsed, nil
}
