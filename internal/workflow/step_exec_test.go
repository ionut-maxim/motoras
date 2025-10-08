package workflow

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
)

func TestExec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    *Exec
		wantErr bool
	}{
		{
			name: "valid simple command",
			step: &Exec{
				Name:    "echo_test",
				Command: "echo",
				Args:    []string{"hello"},
			},
			wantErr: false,
		},
		{
			name: "valid shell command",
			step: &Exec{
				Name:    "shell_test",
				Command: "echo hello | cat",
				Shell:   true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			step: &Exec{
				Command: "echo",
			},
			wantErr: true,
		},
		{
			name: "missing command",
			step: &Exec{
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
				t.Errorf("Exec.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExec_Execute_SimpleCommand(t *testing.T) {
	step := &Exec{
		Name:    "echo_test",
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	// Check that response was stored in env
	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}

	if response.Stdout != "hello world\n" {
		t.Errorf("Expected stdout='hello world\\n', got %q", response.Stdout)
	}

	if response.Stderr != "" {
		t.Errorf("Expected empty stderr, got %q", response.Stderr)
	}
}

func TestExec_Execute_ShellCommand(t *testing.T) {
	step := &Exec{
		Name:    "shell_test",
		Command: "echo hello | tr 'a-z' 'A-Z'",
		Shell:   true,
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}

	if response.Stdout != "HELLO\n" {
		t.Errorf("Expected stdout='HELLO\\n', got %q", response.Stdout)
	}
}

func TestExec_Execute_JSONOutput(t *testing.T) {
	// Use a command that outputs JSON
	step := &Exec{
		Name:    "json_test",
		Command: "echo",
		Args:    []string{`{"message":"success","count":42}`},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
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

	// JSON numbers are decoded as float64
	if output["count"] != float64(42) {
		t.Errorf("Expected count=42, got %v", output["count"])
	}
}

func TestExec_Execute_WithEnv(t *testing.T) {
	step := &Exec{
		Name:    "env_test",
		Command: "sh",
		Args:    []string{"-c", "echo $MY_VAR"},
		Env: map[string]string{
			"MY_VAR": "test_value",
		},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Stdout != "test_value\n" {
		t.Errorf("Expected stdout='test_value\\n', got %q", response.Stdout)
	}
}

func TestExec_Execute_WithExpressions(t *testing.T) {
	// Set up environment with variables
	env := make(Env)
	env["greeting"] = "hello"
	env["name"] = "world"

	step := &Exec{
		Name:    "expr_test",
		Command: "echo",
		Args:    []string{"{{greeting}}", "{{name}}"},
	}

	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Stdout != "hello world\n" {
		t.Errorf("Expected stdout='hello world\\n', got %q", response.Stdout)
	}
}

func TestExec_Execute_FailingCommand(t *testing.T) {
	// Use a command that will fail
	var step *Exec
	if runtime.GOOS == "windows" {
		step = &Exec{
			Name:    "fail_test",
			Command: "cmd",
			Args:    []string{"/c", "exit 1"},
		}
	} else {
		step = &Exec{
			Name:    "fail_test",
			Command: "sh",
			Args:    []string{"-c", "exit 1"},
		}
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected error for failing command, got nil")
	}

	// Even though command failed, response should be in env
	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", response.ExitCode)
	}

	if response.Error == "" {
		t.Error("Expected Error to be set, got empty string")
	}
}

func TestExec_Execute_CommandNotFound(t *testing.T) {
	step := &Exec{
		Name:    "notfound_test",
		Command: "this-command-does-not-exist-12345",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected error for non-existent command, got nil")
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != -1 {
		t.Errorf("Expected exit code -1 for command not found, got %d", response.ExitCode)
	}
}

func TestExec_Execute_ComplexJSON(t *testing.T) {
	// Create a complex JSON structure
	jsonData := map[string]any{
		"users": []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		},
		"total": 2,
		"meta":  map[string]any{"page": 1},
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	step := &Exec{
		Name:    "complex_json_test",
		Command: "echo",
		Args:    []string{string(jsonBytes)},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	// Check that output was parsed as JSON
	if response.Output == nil {
		t.Fatal("Expected Output to be parsed, got nil")
	}

	output, ok := response.Output.(map[string]any)
	if !ok {
		t.Fatalf("Expected output to be map[string]any, got %T", response.Output)
	}

	if output["total"] != float64(2) {
		t.Errorf("Expected total=2, got %v", output["total"])
	}

	users, ok := output["users"].([]any)
	if !ok {
		t.Fatalf("Expected users to be []any, got %T", output["users"])
	}

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestExec_Execute_NonJSONOutput(t *testing.T) {
	step := &Exec{
		Name:    "non_json_test",
		Command: "echo",
		Args:    []string{"this is not json"},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	// For non-JSON output, Output should be nil
	if response.Output != nil {
		t.Errorf("Expected Output to be nil for non-JSON output, got %v", response.Output)
	}

	// But Stdout should contain the output
	if response.Stdout != "this is not json\n" {
		t.Errorf("Expected stdout='this is not json\\n', got %q", response.Stdout)
	}
}

func TestExec_Isolation_NetworkDisabled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Network isolation is only supported on Linux")
	}

	step := &Exec{
		Name:    "network_test",
		Command: "ping",
		Args:    []string{"-c", "1", "8.8.8.8"},
		Isolation: &ExecIsolation{
			DisableNetwork: true,
		},
		Timeout: 2, // Short timeout since it should fail
	}

	env := make(Env)
	ctx := context.Background()

	// Command should fail because network is disabled
	_, err := step.Execute(ctx, env)
	if err == nil {
		t.Error("Expected error when network is disabled, got nil")
	}
}

func TestExec_Isolation_PIDNamespace(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PID namespace isolation is only supported on Linux")
	}

	step := &Exec{
		Name:    "pid_namespace_test",
		Command: "sh",
		Args:    []string{"-c", "echo $$"}, // Print PID
		Isolation: &ExecIsolation{
			NewPIDNamespace: true,
		},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	// In a new PID namespace, the process should see itself as PID 1
	// (though this might vary depending on how the shell runs)
	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}
}

func TestExec_Isolation_MemoryLimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Memory limits are only supported on Linux")
	}

	// Try to allocate more memory than the limit
	// This might not always trigger on all systems
	step := &Exec{
		Name:    "memory_limit_test",
		Command: "sh",
		Args:    []string{"-c", "echo 'test'"},
		Isolation: &ExecIsolation{
			MemoryLimitMB: 10, // Very low limit
		},
	}

	env := make(Env)
	ctx := context.Background()

	// The command should still work for simple operations
	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}
}

func TestExec_Isolation_UnsupportedPlatform(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("This test is for non-Linux platforms")
	}

	step := &Exec{
		Name:    "isolation_test",
		Command: "echo",
		Args:    []string{"test"},
		Isolation: &ExecIsolation{
			DisableNetwork: true,
		},
	}

	env := make(Env)
	ctx := context.Background()

	// Should fail on non-Linux platforms
	_, err := step.Execute(ctx, env)
	if err == nil {
		t.Error("Expected error for isolation on non-Linux platform, got nil")
	}

	if err != nil && err.Error() != "failed to apply isolation: process and network isolation is only supported on Linux" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestExec_Isolation_CombinedNamespaces(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Namespace isolation is only supported on Linux")
	}

	step := &Exec{
		Name:    "combined_isolation_test",
		Command: "hostname",
		Isolation: &ExecIsolation{
			NewPIDNamespace: true,
			NewIPCNamespace: true,
			NewUTSNamespace: true,
		},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("Exec.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(ExecResponse)
	if !ok {
		t.Fatalf("Expected ExecResponse in env, got %T", resultEnv[step.Name])
	}

	if response.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.ExitCode)
	}
}
