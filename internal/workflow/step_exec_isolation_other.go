//go:build !linux

package workflow

import (
	"errors"
	"os/exec"
)

func applyIsolation(cmd *exec.Cmd, isolation *ExecIsolation) error {
	// Check if any isolation features are requested
	if isolation.DisableNetwork || isolation.NewPIDNamespace ||
		isolation.NewIPCNamespace || isolation.NewUTSNamespace ||
		isolation.RunAsUser != "" || isolation.RunAsGroup != "" ||
		isolation.MemoryLimitMB > 0 || isolation.CPUQuota > 0 {
		return errors.New("process and network isolation is only supported on Linux")
	}

	return nil
}

func validateIsolationSupport(isolation *ExecIsolation) error {
	return applyIsolation(nil, isolation)
}
