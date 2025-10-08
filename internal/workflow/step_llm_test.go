package workflow

import (
	"context"
	"testing"
)

func TestLLM_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    *LLM
		wantErr bool
	}{
		{
			name: "valid llm step",
			step: &LLM{
				Name:     "test_llm",
				Provider: "mock",
				Model:    "gpt-4",
				Prompt:   "What is 2+2?",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			step: &LLM{
				Provider: "mock",
				Model:    "gpt-4",
				Prompt:   "test",
			},
			wantErr: true,
		},
		{
			name: "missing provider",
			step: &LLM{
				Name:   "test",
				Model:  "gpt-4",
				Prompt: "test",
			},
			wantErr: true,
		},
		{
			name: "missing model",
			step: &LLM{
				Name:     "test",
				Provider: "mock",
				Prompt:   "test",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			step: &LLM{
				Name:     "test",
				Provider: "mock",
				Model:    "gpt-4",
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			step: &LLM{
				Name:     "test",
				Provider: "invalid",
				Model:    "gpt-4",
				Prompt:   "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := make(Env)
			err := tt.step.Validate(env)
			if (err != nil) != tt.wantErr {
				t.Errorf("LLM.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLLM_Execute_Mock(t *testing.T) {
	step := &LLM{
		Name:     "mock_test",
		Provider: "mock",
		Model:    "mock-model",
		Prompt:   "This is a test prompt",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}

	// Check that response was stored in env
	response, ok := resultEnv[step.Name].(LLMResponse)
	if !ok {
		t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Provider != "mock" {
		t.Errorf("Expected provider='mock', got %q", response.Provider)
	}

	if response.Response == "" {
		t.Error("Expected non-empty response")
	}

	if response.TokensUsed != 42 {
		t.Errorf("Expected tokens_used=42, got %d", response.TokensUsed)
	}
}

func TestLLM_Execute_JSONParsing(t *testing.T) {
	step := &LLM{
		Name:     "json_test",
		Provider: "mock",
		Model:    "mock-model",
		Prompt:   "Give me a structured JSON response",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(LLMResponse)
	if !ok {
		t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
	}

	// Mock should return JSON for "json" keyword
	if response.Output == nil {
		t.Fatal("Expected Output to be parsed, got nil")
	}

	output, ok := response.Output.(map[string]any)
	if !ok {
		t.Fatalf("Expected output to be map[string]any, got %T", response.Output)
	}

	if output["status"] != "success" {
		t.Errorf("Expected status='success', got %v", output["status"])
	}
}

func TestLLM_Execute_WithExpressions(t *testing.T) {
	// Set up environment with variables
	env := make(Env)
	env["user_input"] = "What is the weather?"
	env["api_key"] = "secret-key-123"

	step := &LLM{
		Name:     "expr_test",
		Provider: "mock",
		Model:    "mock-model",
		Prompt:   "User asked: {{user_input}}",
		APIKey:   "{{api_key}}",
	}

	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(LLMResponse)
	if !ok {
		t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Prompt != "User asked: What is the weather?" {
		t.Errorf("Expected evaluated prompt, got %q", response.Prompt)
	}
}

func TestLLM_Execute_CustomSystemPrompt(t *testing.T) {
	customPrompt := "You are a helpful assistant that only speaks in haiku."

	step := &LLM{
		Name:         "custom_prompt",
		Provider:     "mock",
		Model:        "mock-model",
		Prompt:       "Tell me about Go",
		SystemPrompt: customPrompt,
	}

	env := make(Env)
	ctx := context.Background()

	_, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}

	// Mock implementation doesn't use system prompt, but we verify it doesn't error
}

func TestLLM_Execute_Temperature(t *testing.T) {
	step := &LLM{
		Name:        "temp_test",
		Provider:    "mock",
		Model:       "mock-model",
		Prompt:      "Be creative",
		Temperature: 1.5,
	}

	env := make(Env)
	ctx := context.Background()

	_, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}
}

func TestLLM_Execute_MockError(t *testing.T) {
	step := &LLM{
		Name:     "error_test",
		Provider: "mock",
		Model:    "mock-model",
		Prompt:   "This should trigger an error",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected error for error trigger, got nil")
	}

	// Even though execution failed, response should be in env
	response, ok := resultEnv[step.Name].(LLMResponse)
	if !ok {
		t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
	}

	if response.Error == "" {
		t.Error("Expected error field to be populated")
	}
}

func TestLLM_Execute_DecisionMaking(t *testing.T) {
	step := &LLM{
		Name:     "decision_test",
		Provider: "mock",
		Model:    "mock-model",
		Prompt:   "Should we approve this request? Make a decision.",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("LLM.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(LLMResponse)
	if !ok {
		t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
	}

	// Mock should return decision JSON for "decision" keyword
	if response.Output == nil {
		t.Fatal("Expected Output to be parsed, got nil")
	}

	output, ok := response.Output.(map[string]any)
	if !ok {
		t.Fatalf("Expected output to be map[string]any, got %T", response.Output)
	}

	if output["decision"] != "approve" {
		t.Errorf("Expected decision='approve', got %v", output["decision"])
	}

	if output["confidence"] == nil {
		t.Error("Expected confidence field in output")
	}
}

func TestLLM_Execute_MissingAPIKey(t *testing.T) {
	// Test that providers require API keys
	tests := []struct {
		name     string
		provider string
	}{
		{"openai_no_key", "openai"},
		{"anthropic_no_key", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &LLM{
				Name:     "api_key_test",
				Provider: tt.provider,
				Model:    "test-model",
				Prompt:   "test",
				// No API key provided
			}

			env := make(Env)
			ctx := context.Background()

			resultEnv, err := step.Execute(ctx, env)
			if err == nil {
				t.Fatal("Expected error for missing API key")
			}

			// Should still have response in env
			response, ok := resultEnv[step.Name].(LLMResponse)
			if !ok {
				t.Fatalf("Expected LLMResponse in env, got %T", resultEnv[step.Name])
			}

			if response.Error == "" {
				t.Error("Expected error message in response")
			}
		})
	}
}

func TestLLM_GetName(t *testing.T) {
	step := &LLM{Name: "my_llm"}
	if step.GetName() != "my_llm" {
		t.Errorf("Expected name='my_llm', got %q", step.GetName())
	}
}
