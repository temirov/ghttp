package certificates

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// CommandRunner executes system commands.
type CommandRunner interface {
	Run(ctx context.Context, executable string, arguments []string) error
	RunWithPrivileges(ctx context.Context, executable string, arguments []string) error
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

// RunWithPrivileges executes the command with elevated privileges when supported.
func (executableRunner ExecutableRunner) RunWithPrivileges(ctx context.Context, executable string, arguments []string) error {
	switch runtime.GOOS {
	case "darwin", "linux":
		privilegedArguments := append([]string{executable}, arguments...)
		return executableRunner.Run(ctx, "sudo", privilegedArguments)
	default:
		return fmt.Errorf("privileged execution not supported on %s", runtime.GOOS)
	}
}
