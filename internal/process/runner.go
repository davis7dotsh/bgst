package process

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct{}

func (Runner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir

	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return "", fmt.Errorf("%s: %s", name, text)
	}

	return text, nil
}
