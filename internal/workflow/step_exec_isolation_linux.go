//go:build linux

package workflow

import (
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func applyIsolation(cmd *exec.Cmd, isolation *ExecIsolation) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Apply namespace isolation
	var cloneFlags uintptr

	if isolation.DisableNetwork {
		// Create new network namespace (isolated from host network)
		cloneFlags |= syscall.CLONE_NEWNET
	}

	if isolation.NewPIDNamespace {
		// Create new PID namespace (process isolation)
		cloneFlags |= syscall.CLONE_NEWPID
	}

	if isolation.NewIPCNamespace {
		// Create new IPC namespace (isolate IPC resources)
		cloneFlags |= syscall.CLONE_NEWIPC
	}

	if isolation.NewUTSNamespace {
		// Create new UTS namespace (isolate hostname/domain)
		cloneFlags |= syscall.CLONE_NEWUTS
	}

	if cloneFlags != 0 {
		cmd.SysProcAttr.Cloneflags = cloneFlags
	}

	// Apply user/group isolation
	if isolation.RunAsUser != "" || isolation.RunAsGroup != "" {
		cred := &syscall.Credential{}

		if isolation.RunAsUser != "" {
			uid, gid, err := lookupUser(isolation.RunAsUser)
			if err != nil {
				return fmt.Errorf("failed to lookup user %s: %w", isolation.RunAsUser, err)
			}
			cred.Uid = uid
			// Use user's primary group if no group specified
			if isolation.RunAsGroup == "" {
				cred.Gid = gid
			}
		}

		if isolation.RunAsGroup != "" {
			gid, err := lookupGroup(isolation.RunAsGroup)
			if err != nil {
				return fmt.Errorf("failed to lookup group %s: %w", isolation.RunAsGroup, err)
			}
			cred.Gid = gid
		}

		cmd.SysProcAttr.Credential = cred
	}

	// Apply resource limits
	if isolation.MemoryLimitMB > 0 || isolation.CPUQuota > 0 {
		// Note: Resource limits via cgroups require more complex setup
		// For now, we'll use setrlimit which provides basic limits
		if cmd.SysProcAttr.Rlimit == nil {
			cmd.SysProcAttr.Rlimit = make([]syscall.Rlimit, 0)
		}

		if isolation.MemoryLimitMB > 0 {
			// Set memory limit (AS = address space)
			memLimit := uint64(isolation.MemoryLimitMB * 1024 * 1024)
			cmd.SysProcAttr.Rlimit = append(cmd.SysProcAttr.Rlimit, syscall.Rlimit{
				Resource: syscall.RLIMIT_AS,
				Cur:      memLimit,
				Max:      memLimit,
			})
		}

		// CPU quota is harder to enforce with just rlimit
		// Would need cgroup v2 integration for proper CPU quota
		if isolation.CPUQuota > 0 {
			// CPU time limit (not a hard quota, just max CPU time)
			// This is a basic approximation
			cpuTimeLimit := uint64(isolation.CPUQuota)
			cmd.SysProcAttr.Rlimit = append(cmd.SysProcAttr.Rlimit, syscall.Rlimit{
				Resource: syscall.RLIMIT_CPU,
				Cur:      cpuTimeLimit,
				Max:      cpuTimeLimit,
			})
		}
	}

	return nil
}

// lookupUser looks up a user by name or numeric UID
func lookupUser(userName string) (uint32, uint32, error) {
	// Try parsing as numeric UID first
	if uid, err := strconv.ParseUint(userName, 10, 32); err == nil {
		u, err := user.LookupId(strconv.FormatUint(uid, 10))
		if err != nil {
			// UID exists but can't get details, use UID with GID 0
			return uint32(uid), 0, nil
		}
		gid, _ := strconv.ParseUint(u.Gid, 10, 32)
		return uint32(uid), uint32(gid), nil
	}

	// Look up by username
	u, err := user.Lookup(userName)
	if err != nil {
		return 0, 0, err
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, err
	}

	return uint32(uid), uint32(gid), nil
}

// lookupGroup looks up a group by name or numeric GID
func lookupGroup(groupName string) (uint32, error) {
	// Try parsing as numeric GID first
	if gid, err := strconv.ParseUint(groupName, 10, 32); err == nil {
		// Verify GID exists
		if _, err := user.LookupGroupId(strconv.FormatUint(gid, 10)); err != nil {
			return 0, err
		}
		return uint32(gid), nil
	}

	// Look up by group name
	g, err := user.LookupGroup(groupName)
	if err != nil {
		return 0, err
	}

	gid, err := strconv.ParseUint(g.Gid, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(gid), nil
}

// validateIsolationSupport checks if isolation features are supported
func validateIsolationSupport(isolation *ExecIsolation) error {
	// On Linux, check if we have necessary capabilities
	if isolation.DisableNetwork || isolation.NewPIDNamespace ||
		isolation.NewIPCNamespace || isolation.NewUTSNamespace {
		// Namespace operations typically require CAP_SYS_ADMIN
		// We can't easily check capabilities in pure Go without cgo
		// So we'll just proceed and let the kernel reject if needed
	}

	if isolation.RunAsUser != "" || isolation.RunAsGroup != "" {
		// Check if running as root (needed to change to different user)
		if syscall.Getuid() != 0 {
			return errors.New("changing user/group requires root privileges")
		}
	}

	return nil
}
