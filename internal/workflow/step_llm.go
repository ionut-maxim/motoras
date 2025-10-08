package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go"
	openaiopt "github.com/openai/openai-go/option"

	"github.com/ionut-maxim/motoras/internal/expression"
)

const defaultSystemPrompt = `You are an AI agent operating within a workflow orchestration engine called Motoraș.

Your role is to process data and make decisions as part of automated workflows. Your responses will be parsed and used by subsequent workflow steps.

IMPORTANT GUIDELINES:
1. Always provide your response as valid JSON when possible
2. Structure your output to be machine-readable
3. Be concise and direct - avoid unnecessary explanations
4. Focus on the task at hand without preamble or postamble
5. When asked for a decision, provide it in a clear, parseable format

Example of good structured output:
{
  "decision": "approve",
  "confidence": 0.95,
  "reason": "All validation checks passed"
}

Your responses will be available to other workflow steps through the environment context.`

type LLM struct {
	Name         string            `json:"name"`
	Provider     string            `json:"provider"`                                // openai, anthropic, mock
	Model        string            `json:"model"`                                   // e.g., gpt-4, claude-3-opus-20240229
	Prompt       string            `json:"prompt" expression:"template"`            // User prompt with template support
	SystemPrompt string            `json:"system_prompt,omitempty"`                 // Optional override of default system prompt
	Temperature  float32           `json:"temperature,omitempty"`                   // 0.0 - 2.0, default 0.7
	MaxTokens    int               `json:"max_tokens,omitempty"`                    // Maximum tokens to generate
	APIKey       string            `json:"api_key,omitempty" expression:"template"` // API key (use with {{secrets.api_key}})
	Config       map[string]string `json:"config,omitempty"`                        // Provider-specific configuration
}

type LLMResponse struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	Response     string `json:"response"`
	Output       any    `json:"output,omitempty"`        // Parsed JSON if response is valid JSON
	FinishReason string `json:"finish_reason,omitempty"` // stop, length, etc.
	TokensUsed   int    `json:"tokens_used,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (l *LLM) GetName() string {
	return l.Name
}

func (l *LLM) Validate(env Env) error {
	if l.Name == "" {
		return errors.New("missing step name")
	}
	if l.Provider == "" {
		return errors.New("missing provider")
	}
	if l.Model == "" {
		return errors.New("missing model")
	}
	if l.Prompt == "" {
		return errors.New("missing prompt")
	}

	// Validate provider
	validProviders := map[string]bool{
		"openai":    true,
		"anthropic": true,
		"mock":      true, // For testing
	}
	if !validProviders[l.Provider] {
		return fmt.Errorf("invalid provider: %s (must be openai, anthropic, or mock)", l.Provider)
	}

	return env.new(l.Name)
}

func (l *LLM) Execute(ctx context.Context, env Env) (Env, error) {
	// Evaluate expressions
	evaluated, err := expression.Parse(ctx, l, map[string]any(env))
	if err != nil {
		return env, fmt.Errorf("failed to evaluate expressions: %w", err)
	}

	// Use default system prompt if not provided
	systemPrompt := evaluated.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	// Set default temperature
	temperature := evaluated.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	// Execute based on provider
	var response LLMResponse
	switch evaluated.Provider {
	case "openai":
		response, err = l.executeOpenAI(ctx, evaluated, systemPrompt, temperature)
	case "anthropic":
		response, err = l.executeAnthropic(ctx, evaluated, systemPrompt, temperature)
	case "mock":
		response, err = l.executeMock(ctx, evaluated, systemPrompt, temperature)
	default:
		return env, fmt.Errorf("unsupported provider: %s", evaluated.Provider)
	}

	if err != nil {
		response.Error = err.Error()
		env[l.Name] = response
		return env, err
	}

	// Try to parse response as JSON
	if response.Response != "" {
		trimmed := strings.TrimSpace(response.Response)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			var parsed any
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				response.Output = parsed
			}
		}
	}

	// Store response in environment
	env[l.Name] = response
	return env, nil
}

// executeMock provides a mock implementation for testing
func (l *LLM) executeMock(ctx context.Context, evaluated *LLM, systemPrompt string, temperature float32) (LLMResponse, error) {
	// Mock responses based on prompt content for testing
	response := LLMResponse{
		Provider:     "mock",
		Model:        evaluated.Model,
		Prompt:       evaluated.Prompt,
		FinishReason: "stop",
		TokensUsed:   42,
	}

	// Generate mock response based on prompt keywords
	prompt := strings.ToLower(evaluated.Prompt)
	if strings.Contains(prompt, "json") || strings.Contains(prompt, "structured") {
		response.Response = `{"status": "success", "message": "Mock structured response", "data": {"value": 42}}`
	} else if strings.Contains(prompt, "approve") || strings.Contains(prompt, "decision") {
		response.Response = `{"decision": "approve", "confidence": 0.95, "reason": "Mock approval decision"}`
	} else if strings.Contains(prompt, "error") || strings.Contains(prompt, "fail") {
		return response, errors.New("mock error response")
	} else {
		response.Response = "This is a mock LLM response for testing purposes."
	}

	return response, nil
}

// executeOpenAI handles OpenAI API calls
func (l *LLM) executeOpenAI(ctx context.Context, evaluated *LLM, systemPrompt string, temperature float32) (LLMResponse, error) {
	response := LLMResponse{
		Provider: "openai",
		Model:    evaluated.Model,
		Prompt:   evaluated.Prompt,
	}

	// Validate API key
	if evaluated.APIKey == "" {
		return response, errors.New("API key is required for OpenAI provider")
	}

	// Create OpenAI client
	client := openai.NewClient(openaiopt.WithAPIKey(evaluated.APIKey))

	// Build messages
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(evaluated.Prompt),
	}

	// Set default max_tokens if not provided
	maxTokens := int64(evaluated.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 1000 // Reasonable default
	}

	// Create chat completion request
	params := openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       openai.ChatModel(evaluated.Model),
		Temperature: openai.Float(float64(temperature)),
		MaxTokens:   openai.Int(maxTokens),
	}

	// Make API call
	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		response.Error = err.Error()
		return response, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Extract response
	if len(completion.Choices) == 0 {
		return response, errors.New("OpenAI returned no choices")
	}

	choice := completion.Choices[0]
	response.Response = choice.Message.Content
	response.FinishReason = string(choice.FinishReason)
	response.TokensUsed = int(completion.Usage.TotalTokens)

	return response, nil
}

// executeAnthropic handles Anthropic Claude API calls
func (l *LLM) executeAnthropic(ctx context.Context, evaluated *LLM, systemPrompt string, temperature float32) (LLMResponse, error) {
	response := LLMResponse{
		Provider: "anthropic",
		Model:    evaluated.Model,
		Prompt:   evaluated.Prompt,
	}

	// Validate API key
	if evaluated.APIKey == "" {
		return response, errors.New("API key is required for Anthropic provider")
	}

	// Create Anthropic client
	client := anthropic.NewClient(option.WithAPIKey(evaluated.APIKey))

	// Set default max_tokens if not provided
	maxTokens := int64(evaluated.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 1000 // Reasonable default
	}

	// Create message request
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(evaluated.Model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(evaluated.Prompt)),
		},
		System: []anthropic.TextBlockParam{
			{
				Text: systemPrompt,
				Type: "text",
			},
		},
		Temperature: anthropic.Float(float64(temperature)),
	}

	// Make API call
	message, err := client.Messages.New(ctx, params)
	if err != nil {
		response.Error = err.Error()
		return response, fmt.Errorf("Anthropic API call failed: %w", err)
	}

	// Extract response
	if len(message.Content) == 0 {
		return response, errors.New("Anthropic returned no content")
	}

	// Concatenate all text blocks
	var contentBuilder strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			contentBuilder.WriteString(block.Text)
		}
	}

	response.Response = contentBuilder.String()
	response.FinishReason = string(message.StopReason)
	response.TokensUsed = int(message.Usage.InputTokens + message.Usage.OutputTokens)

	return response, nil
}
