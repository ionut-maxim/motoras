package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ionut-maxim/motoras/internal/expression"
)

type Exec struct {
	Name       string            `json:"name"`
	Command    string            `json:"command" expression:"template"`
	Args       []string          `json:"args,omitempty"`
	Shell      bool              `json:"shell,omitempty"` // If true, run command through shell
	WorkingDir string            `json:"working_dir,omitempty" expression:"template"`
	Env        map[string]string `json:"env,omitempty"`     // Additional environment variables
	Timeout    int               `json:"timeout,omitempty"` // Timeout in seconds, default 60
	Isolation  *ExecIsolation    `json:"isolation,omitempty"`
}

// ExecIsolation defines process and network isolation settings
type ExecIsolation struct {
	// Network isolation
	DisableNetwork bool `json:"disable_network,omitempty"` // Disable network access (Linux only)

	// Process isolation
	NewPIDNamespace bool `json:"new_pid_namespace,omitempty"` // Run in new PID namespace (Linux only)
	NewIPCNamespace bool `json:"new_ipc_namespace,omitempty"` // Run in new IPC namespace (Linux only)
	NewUTSNamespace bool `json:"new_uts_namespace,omitempty"` // Run in new UTS namespace (Linux only)

	// User/Group isolation
	RunAsUser  string `json:"run_as_user,omitempty"`  // User to run as (username or UID)
	RunAsGroup string `json:"run_as_group,omitempty"` // Group to run as (group name or GID)

	// Resource limits
	MemoryLimitMB int64 `json:"memory_limit_mb,omitempty"` // Memory limit in MB
	CPUQuota      int64 `json:"cpu_quota,omitempty"`       // CPU quota in percentage (100 = 1 core)
}

type ExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Output   any    `json:"output,omitempty"` // Parsed JSON output if stdout is valid JSON
	Error    string `json:"error,omitempty"`
}

func (e *Exec) GetName() string {
	return e.Name
}

func (e *Exec) Validate(env Env) error {
	if e.Name == "" {
		return errors.New("missing step name")
	}
	if e.Command == "" {
		return errors.New("missing command")
	}
	return env.new(e.Name)
}

func (e *Exec) Execute(ctx context.Context, env Env) (Env, error) {
	// Evaluate expressions in the Exec step using the current environment
	evaluated, err := expression.Parse(ctx, e, map[string]any(env))
	if err != nil {
		return env, fmt.Errorf("failed to evaluate expressions: %w", err)
	}

	// Evaluate args if they contain templates
	evaluatedArgs := make([]string, len(evaluated.Args))
	for i, arg := range evaluated.Args {
		type argValue struct {
			Value string `expression:"template"`
		}
		av := argValue{Value: arg}
		evaluatedAV, evalErr := expression.Parse(ctx, &av, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate arg[%d]: %w", i, evalErr)
		}
		evaluatedArgs[i] = evaluatedAV.Value
	}

	// Configure timeout
	timeout := time.Duration(evaluated.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	var cmd *exec.Cmd
	if evaluated.Shell {
		// Run through shell
		shellCmd := evaluated.Command
		if len(evaluatedArgs) > 0 {
			// Join command and args into a single shell command
			shellCmd = fmt.Sprintf("%s %s", evaluated.Command, strings.Join(evaluatedArgs, " "))
		}
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", shellCmd)
	} else {
		// Run command directly
		cmd = exec.CommandContext(cmdCtx, evaluated.Command, evaluatedArgs...)
	}

	// Set working directory if provided
	if evaluated.WorkingDir != "" {
		cmd.Dir = evaluated.WorkingDir
	}

	// Set environment variables
	if len(evaluated.Env) > 0 {
		// Start with current environment
		cmd.Env = append(cmd.Env, cmd.Environ()...)

		// Evaluate and add custom environment variables
		for key, value := range evaluated.Env {
			type envValue struct {
				Value string `expression:"template"`
			}
			ev := envValue{Value: value}
			evaluatedEV, evalErr := expression.Parse(ctx, &ev, map[string]any(env))
			if evalErr != nil {
				return env, fmt.Errorf("failed to evaluate env var %s: %w", key, evalErr)
			}
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, evaluatedEV.Value))
		}
	}

	// Apply isolation settings if configured
	if evaluated.Isolation != nil {
		if err := applyIsolation(cmd, evaluated.Isolation); err != nil {
			return env, fmt.Errorf("failed to apply isolation: %w", err)
		}
	}

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	execErr := cmd.Run()

	// Prepare response
	response := ExecResponse{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	// Get exit code
	if execErr != nil {
		response.Error = execErr.Error()
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			response.ExitCode = exitErr.ExitCode()
		} else {
			// Non-exit error (e.g., command not found)
			response.ExitCode = -1
		}
	} else {
		response.ExitCode = 0
	}

	// Try to parse stdout as JSON if command succeeded
	if execErr == nil && stdout.Len() > 0 {
		var parsedOutput any
		if err := json.Unmarshal([]byte(stdout.String()), &parsedOutput); err == nil {
			// Successfully parsed as JSON
			response.Output = parsedOutput
		}
	}

	// Store response in environment
	env[e.Name] = response

	// Return error if command failed
	if execErr != nil {
		return env, fmt.Errorf("command failed with exit code %d: %w", response.ExitCode, execErr)
	}

	return env, nil
}
