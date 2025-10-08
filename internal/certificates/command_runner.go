package certificates

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandRunner executes system commands.
type CommandRunner interface {
	Run(ctx context.Context, executable string, arguments []string) error
}

// ExecutableRunner executes commands using the local operating system.
type ExecutableRunner struct{}

// NewExecutableRunner constructs an ExecutableRunner.
func NewExecutableRunner() ExecutableRunner {
	return ExecutableRunner{}
}

// Run executes the executable with the provided arguments.
func (executableRunner ExecutableRunner) Run(ctx context.Context, executable string, arguments []string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	var stderrBuffer bytes.Buffer
	command.Stderr = &stderrBuffer
	err := command.Run()
	if err != nil {
		return fmt.Errorf("execute %s: %w: %s", executable, err, stderrBuffer.String())
	}
	return nil
}
