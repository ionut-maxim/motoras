package workflow

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

func isDockerAvailable() bool {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()

	_, err = cli.Ping(context.Background())
	return err == nil
}

func TestContainer_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    *Container
		wantErr bool
	}{
		{
			name: "valid container",
			step: &Container{
				Name:  "test_container",
				Image: "alpine:latest",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			step: &Container{
				Image: "alpine:latest",
			},
			wantErr: true,
		},
		{
			name: "missing image",
			step: &Container{
				Name: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := make(Env)
			err := tt.step.Validate(env)
			if (err != nil) != tt.wantErr {
				t.Errorf("Container.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainer_Execute_SimpleCommand(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:    "echo_test",
		Image:   "alpine:latest",
		Command: []string{"echo"},
		Args:    []string{"hello", "world"},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Container.Execute() error = %v", err)
	}

	// Check that response was stored in env
	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}

	if response.Logs != "hello world\n" {
		t.Errorf("Expected logs='hello world\\n', got %q", response.Logs)
	}
}

func TestContainer_Execute_WithEnv(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:    "env_test",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c"},
		Args:    []string{"echo $MY_VAR"},
		Env: map[string]string{
			"MY_VAR": "test_value",
		},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Container.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Logs != "test_value\n" {
		t.Errorf("Expected logs='test_value\\n', got %q", response.Logs)
	}
}

func TestContainer_Execute_JSONOutput(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:    "json_test",
		Image:   "alpine:latest",
		Command: []string{"echo"},
		Args:    []string{`{"message":"success","count":42}`},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Container.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	// Check that output was parsed as JSON
	if response.Output == nil {
		t.Fatal("Expected Output to be parsed, got nil")
	}

	output, ok := response.Output.(map[string]any)
	if !ok {
		t.Fatalf("Expected output to be map[string]any, got %T", response.Output)
	}

	if output["message"] != "success" {
		t.Errorf("Expected message='success', got %v", output["message"])
	}

	if output["count"] != float64(42) {
		t.Errorf("Expected count=42, got %v", output["count"])
	}
}

func TestContainer_Execute_WithExpressions(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	// Set up environment with variables
	env := make(Env)
	env["greeting"] = "hello"
	env["name"] = "docker"

	step := &Container{
		Name:    "expr_test",
		Image:   "alpine:latest",
		Command: []string{"echo"},
		Args:    []string{"{{greeting}}", "{{name}}"},
	}

	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Container.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Logs != "hello docker\n" {
		t.Errorf("Expected logs='hello docker\\n', got %q", response.Logs)
	}
}

func TestContainer_Execute_FailingCommand(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:    "fail_test",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c"},
		Args:    []string{"exit 1"},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected error for failing command, got nil")
	}

	// Even though command failed, response should be in env
	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", response.ExitCode)
	}
}

func TestContainer_Execute_Timeout(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:    "timeout_test",
		Image:   "alpine:latest",
		Command: []string{"sleep"},
		Args:    []string{"30"},
		Timeout: 1, // 1 second timeout
	}

	env := make(Env)
	ctx := context.Background()

	_, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if err.Error() != "container execution timed out" {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestContainer_Execute_WithResourceLimits(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	step := &Container{
		Name:          "resource_test",
		Image:         "alpine:latest",
		Command:       []string{"echo"},
		Args:          []string{"test"},
		MemoryLimitMB: 64,
		CPUShares:     512,
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Container.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ContainerResponse)
	if !ok {
		t.Fatalf("Expected ContainerResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}
}

func TestContainer_Execute_PullPolicy(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker is not available")
	}

	tests := []struct {
		name       string
		pullPolicy string
		wantErr    bool
	}{
		{
			name:       "pull_policy_missing",
			pullPolicy: "missing",
			wantErr:    false,
		},
		{
			name:       "pull_policy_never",
			pullPolicy: "never",
			wantErr:    false, // alpine:latest should already be pulled from previous tests
		},
		{
			name:       "invalid_pull_policy",
			pullPolicy: "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &Container{
				Name:       "pull_policy_test",
				Image:      "alpine:latest",
				Command:    []string{"echo"},
				Args:       []string{"test"},
				PullPolicy: tt.pullPolicy,
			}

			env := make(Env)
			ctx := context.Background()

			_, err := step.Execute(ctx, env)
			if (err != nil) != tt.wantErr {
				t.Errorf("Container.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
