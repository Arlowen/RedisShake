//go:build windows

package engine

import "os/exec"

func configureCommand(_ *exec.Cmd) {}

func signalGraceful(command *exec.Cmd) error {
	return command.Process.Kill()
}

func signalForce(command *exec.Cmd) error {
	return command.Process.Kill()
}
