package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/ionut-maxim/motoras/internal/expression"
)

type Container struct {
	Name    string            `json:"name"`
	Image   string            `json:"image" expression:"template"`
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`     // Environment variables
	Volumes []VolumeMount     `json:"volumes,omitempty"` // Volume mounts
	Network string            `json:"network,omitempty"` // Network mode (bridge, host, none)
	Timeout int               `json:"timeout,omitempty"` // Timeout in seconds, default 300 (5 minutes)

	// Resource limits
	MemoryLimitMB int64 `json:"memory_limit_mb,omitempty"` // Memory limit in MB
	CPUShares     int64 `json:"cpu_shares,omitempty"`      // CPU shares (relative weight)

	// Pull policy
	PullPolicy string `json:"pull_policy,omitempty"` // always, missing, never (default: missing)

	// Additional options
	AutoRemove bool `json:"auto_remove,omitempty"` // Remove container after execution (default: true)
	Privileged bool `json:"privileged,omitempty"`  // Run in privileged mode
}

type VolumeMount struct {
	Source   string `json:"source" expression:"template"` // Host path or volume name
	Target   string `json:"target" expression:"template"` // Container path
	ReadOnly bool   `json:"read_only,omitempty"`          // Mount as read-only
}

type ContainerResponse struct {
	ContainerID string `json:"container_id"`
	ExitCode    int64  `json:"exit_code"`
	Logs        string `json:"logs"`
	Output      any    `json:"output,omitempty"` // Parsed JSON if logs are valid JSON
	Error       string `json:"error,omitempty"`
}

func (c *Container) GetName() string {
	return c.Name
}

func (c *Container) Validate(env Env) error {
	if c.Name == "" {
		return errors.New("missing step name")
	}
	if c.Image == "" {
		return errors.New("missing image")
	}
	return env.new(c.Name)
}

func (c *Container) Execute(ctx context.Context, env Env) (Env, error) {
	// Evaluate expressions
	evaluated, err := expression.Parse(ctx, c, map[string]any(env))
	if err != nil {
		return env, fmt.Errorf("failed to evaluate expressions: %w", err)
	}

	// Evaluate command and args if they contain templates
	evaluatedCommand := make([]string, len(evaluated.Command))
	for i, cmd := range evaluated.Command {
		type cmdValue struct {
			Value string `expression:"template"`
		}
		cv := cmdValue{Value: cmd}
		evaluatedCV, evalErr := expression.Parse(ctx, &cv, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate command[%d]: %w", i, evalErr)
		}
		evaluatedCommand[i] = evaluatedCV.Value
	}

	evaluatedArgs := make([]string, len(evaluated.Args))
	for i, arg := range evaluated.Args {
		type argValue struct {
			Value string `expression:"template"`
		}
		av := argValue{Value: arg}
		evaluatedAV, evalErr := expression.Parse(ctx, &av, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate args[%d]: %w", i, evalErr)
		}
		evaluatedArgs[i] = evaluatedAV.Value
	}

	// Configure timeout
	timeout := time.Duration(evaluated.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute // Default 5 minutes
	}

	containerCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return env, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Handle image pull policy
	pullPolicy := evaluated.PullPolicy
	if pullPolicy == "" {
		pullPolicy = "missing" // Default: only pull if missing
	}

	shouldPull := false
	switch pullPolicy {
	case "always":
		shouldPull = true
	case "never":
		shouldPull = false
	case "missing":
		// Check if image exists locally
		_, _, err := cli.ImageInspectWithRaw(containerCtx, evaluated.Image)
		shouldPull = err != nil
	default:
		return env, fmt.Errorf("invalid pull_policy: %s (must be always, missing, or never)", pullPolicy)
	}

	// Pull image if needed
	if shouldPull {
		reader, err := cli.ImagePull(containerCtx, evaluated.Image, image.PullOptions{})
		if err != nil {
			return env, fmt.Errorf("failed to pull image: %w", err)
		}
		defer reader.Close()
		// Drain the pull output
		io.Copy(io.Discard, reader)
	}

	// Prepare environment variables
	envVars := make([]string, 0, len(evaluated.Env))
	for key, value := range evaluated.Env {
		type envValue struct {
			Value string `expression:"template"`
		}
		ev := envValue{Value: value}
		evaluatedEV, evalErr := expression.Parse(ctx, &ev, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate env var %s: %w", key, evalErr)
		}
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, evaluatedEV.Value))
	}

	// Prepare container config
	containerConfig := &container.Config{
		Image: evaluated.Image,
		Env:   envVars,
	}

	// Set command if provided
	if len(evaluatedCommand) > 0 {
		containerConfig.Cmd = append(evaluatedCommand, evaluatedArgs...)
	} else if len(evaluatedArgs) > 0 {
		containerConfig.Cmd = evaluatedArgs
	}

	// Prepare host config
	hostConfig := &container.HostConfig{}

	// Set auto-remove (default true)
	if evaluated.AutoRemove || evaluated.AutoRemove == false && c.AutoRemove {
		hostConfig.AutoRemove = true
	} else {
		hostConfig.AutoRemove = false
	}

	// Set privileged mode
	if evaluated.Privileged {
		hostConfig.Privileged = true
	}

	// Set network mode
	if evaluated.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(evaluated.Network)
	}

	// Set resource limits
	if evaluated.MemoryLimitMB > 0 {
		hostConfig.Resources.Memory = evaluated.MemoryLimitMB * 1024 * 1024
	}
	if evaluated.CPUShares > 0 {
		hostConfig.Resources.CPUShares = evaluated.CPUShares
	}

	// Set volume mounts
	if len(evaluated.Volumes) > 0 {
		binds := make([]string, 0, len(evaluated.Volumes))
		for _, vol := range evaluated.Volumes {
			bind := fmt.Sprintf("%s:%s", vol.Source, vol.Target)
			if vol.ReadOnly {
				bind += ":ro"
			}
			binds = append(binds, bind)
		}
		hostConfig.Binds = binds
	}

	// Create container
	resp, err := cli.ContainerCreate(containerCtx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return env, fmt.Errorf("failed to create container: %w", err)
	}

	containerID := resp.ID

	// Start container
	if err := cli.ContainerStart(containerCtx, containerID, container.StartOptions{}); err != nil {
		return env, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := cli.ContainerWait(containerCtx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	var containerErr error

	select {
	case err := <-errCh:
		if err != nil {
			containerErr = fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
		if status.Error != nil {
			containerErr = fmt.Errorf("container error: %s", status.Error.Message)
		}
	case <-containerCtx.Done():
		// Timeout or cancellation - stop the container
		stopTimeout := 10
		cli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &stopTimeout})
		return env, fmt.Errorf("container execution timed out")
	}

	// Get container logs
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}
	logReader, err := cli.ContainerLogs(containerCtx, containerID, logOptions)
	if err != nil {
		return env, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logReader.Close()

	// Read logs
	logBytes, err := io.ReadAll(logReader)
	if err != nil {
		return env, fmt.Errorf("failed to read container logs: %w", err)
	}

	// Docker logs have 8-byte headers, strip them
	logs := stripDockerLogHeaders(logBytes)

	// Try to parse logs as JSON
	var parsedOutput any
	if exitCode == 0 && len(logs) > 0 {
		if err := json.Unmarshal([]byte(logs), &parsedOutput); err == nil {
			// Successfully parsed as JSON (ignore error if not JSON)
		}
	}

	// Prepare response
	response := ContainerResponse{
		ContainerID: containerID,
		ExitCode:    exitCode,
		Logs:        logs,
		Output:      parsedOutput,
	}

	if containerErr != nil {
		response.Error = containerErr.Error()
	}

	// Store response in environment
	env[c.Name] = response

	// Return error if container failed
	if exitCode != 0 {
		return env, fmt.Errorf("container exited with code %d", exitCode)
	}

	if containerErr != nil {
		return env, containerErr
	}

	return env, nil
}

// stripDockerLogHeaders removes the 8-byte headers from Docker log output
func stripDockerLogHeaders(logBytes []byte) string {
	var result strings.Builder
	i := 0
	for i < len(logBytes) {
		if i+8 > len(logBytes) {
			break
		}
		// Skip the 8-byte header
		// Header format: [stream_type, 0, 0, 0, size1, size2, size3, size4]
		size := int(logBytes[i+4])<<24 | int(logBytes[i+5])<<16 | int(logBytes[i+6])<<8 | int(logBytes[i+7])
		i += 8

		if i+size > len(logBytes) {
			// Malformed log, return what we have
			break
		}

		result.Write(logBytes[i : i+size])
		i += size
	}

	return result.String()
}
