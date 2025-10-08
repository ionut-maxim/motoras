# LLM Step - Large Language Model Prompting

The `llm` step type allows you to integrate Large Language Models into your workflows for intelligent data processing,
decision-making, content generation, and more.

## Features

- ✅ Multiple LLM providers (OpenAI, Anthropic, Mock)
- ✅ Workflow-aware system prompts
- ✅ Automatic JSON parsing of structured outputs
- ✅ Template expression support
- ✅ Configurable temperature and token limits
- ✅ Response stored in workflow environment
- ✅ API key management with secrets support

## Basic Usage

### Simple LLM Prompting

```json
{
  "type": "llm",
  "spec": {
    "name": "content_summary",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Summarize the following text in 3 bullet points: {{input_text}}"
  }
}
```

### Structured Decision Making

```json
{
  "type": "llm",
  "spec": {
    "name": "approval_decision",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Review this request and decide whether to approve it. Request details: {{request_data}}. Respond with JSON containing 'decision', 'confidence', and 'reason' fields."
  }
}
```

The LLM will receive a default system prompt explaining it's an agent in a workflow engine and should provide structured
output.

### With API Key from Secrets

```json
{
  "type": "llm",
  "spec": {
    "name": "openai_call",
    "provider": "openai",
    "model": "gpt-4-turbo",
    "prompt": "Analyze this data: {{data}}",
    "api_key": "{{secrets.openai_api_key}}"
  }
}
```

## Default System Prompt

All LLM steps automatically receive this system prompt (unless overridden):

```
You are an AI agent operating within a workflow orchestration engine called Motoraș.

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

Your responses will be available to other workflow steps through the environment context.
```

This prompt encourages the LLM to:

- Provide JSON responses for easy parsing
- Be concise and workflow-appropriate
- Structure data for downstream consumption

## Advanced Features

### Custom System Prompt

Override the default system prompt for specialized use cases:

```json
{
  "type": "llm",
  "spec": {
    "name": "code_reviewer",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Review this code: {{code}}",
    "system_prompt": "You are an expert code reviewer. Analyze code for bugs, security issues, and best practices. Always respond with JSON containing 'issues', 'severity', and 'recommendations' arrays."
  }
}
```

### Temperature Control

Adjust creativity vs. determinism:

```json
{
  "type": "llm",
  "spec": {
    "name": "creative_writer",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Write a creative product description for: {{product}}",
    "temperature": 1.2
  }
}
```

**Temperature values:**

- `0.0`: Deterministic, focused (best for structured tasks)
- `0.7`: Balanced (default)
- `1.0-2.0`: Creative, varied (best for content generation)

### Token Limits

Control response length:

```json
{
  "type": "llm",
  "spec": {
    "name": "brief_summary",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Summarize: {{long_document}}",
    "max_tokens": 100
  }
}
```

## Response Structure

The LLM step stores a response object in the workflow environment:

```json
{
  "provider": "mock",
  "model": "gpt-4",
  "prompt": "evaluated prompt text",
  "response": "The LLM's text response",
  "output": { "parsed": "json" },
  "finish_reason": "stop",
  "tokens_used": 150,
  "error": "error message if failed"
}
```

### Accessing Response Data

```json
{
  "type": "http",
  "spec": {
    "name": "send_analysis",
    "url": "https://api.example.com/results",
    "method": "POST",
    "body": {
      "llm_response": "{{approval_decision.response}}",
      "parsed_decision": "{{approval_decision.output}}",
      "tokens": "{{approval_decision.tokens_used}}"
    }
  }
}
```

## JSON Output Parsing

Responses that are valid JSON objects or arrays are automatically parsed:

```json
{
  "type": "llm",
  "spec": {
    "name": "data_extractor",
    "provider": "mock",
    "model": "gpt-4",
    "prompt": "Extract key information from this text and return as JSON: {{text}}"
  }
}
```

If the LLM responds with:

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "category": "customer"
}
```

Access the parsed data in subsequent steps:

- `{{data_extractor.output.name}}` → "John Doe"
- `{{data_extractor.output.email}}` → "john@example.com"

## Configuration Options

| Field           | Type              | Description                                  | Default       |
|-----------------|-------------------|----------------------------------------------|---------------|
| `name`          | string            | Step name (required)                         | -             |
| `provider`      | string            | LLM provider (required)                      | -             |
| `model`         | string            | Model identifier (required)                  | -             |
| `prompt`        | string            | User prompt with template support (required) | -             |
| `system_prompt` | string            | Override default system prompt               | default       |
| `temperature`   | float32           | Creativity (0.0-2.0)                         | 0.7           |
| `max_tokens`    | int               | Maximum response tokens                      | model default |
| `api_key`       | string            | API key with template support                | -             |
| `config`        | map[string]string | Provider-specific configuration              | {}            |

### Supported Providers

| Provider    | Status  | Models                                               | API Key Required |
|-------------|---------|------------------------------------------------------|------------------|
| `mock`      | ✅ Ready | Any                                                  | No               |
| `openai`    | ✅ Ready | gpt-4o, gpt-4-turbo, gpt-3.5-turbo, etc.             | Yes              |
| `anthropic` | ✅ Ready | claude-3-opus, claude-3-sonnet, claude-3-haiku, etc. | Yes              |

## Real Provider Examples

### OpenAI Example

```json
{
  "type": "llm",
  "spec": {
    "name": "openai_analysis",
    "provider": "openai",
    "model": "gpt-4o",
    "prompt": "Analyze this code for bugs: {{code}}",
    "api_key": "{{secrets.openai_api_key}}",
    "temperature": 0.2,
    "max_tokens": 1500
  }
}
```

### Anthropic Example

```json
{
  "type": "llm",
  "spec": {
    "name": "claude_review",
    "provider": "anthropic",
    "model": "claude-3-7-sonnet-20250219",
    "prompt": "Review this pull request description and suggest improvements: {{pr_description}}",
    "api_key": "{{secrets.anthropic_api_key}}",
    "temperature": 0.5,
    "max_tokens": 2000
  }
}
```

## Use Cases

### 1. Automated Content Moderation

```json
{
  "type": "llm",
  "spec": {
    "name": "moderate_content",
    "provider": "openai",
    "model": "gpt-4o",
    "prompt": "Review this user comment for policy violations. Comment: '{{user_comment}}'. Respond with JSON: {\"safe\": boolean, \"violations\": [], \"confidence\": number}",
    "api_key": "{{secrets.openai_api_key}}",
    "temperature": 0.3
  }
}
```

### 2. Intelligent Data Extraction

```json
{
  "type": "llm",
  "spec": {
    "name": "extract_invoice_data",
    "provider": "anthropic",
    "model": "claude-3-haiku-20240307",
    "prompt": "Extract invoice details from this text: {{invoice_text}}. Return JSON with fields: invoice_number, date, total, vendor, line_items.",
    "api_key": "{{secrets.anthropic_api_key}}",
    "temperature": 0.0
  }
}
```

### 3. Sentiment Analysis

```json
{
  "type": "llm",
  "spec": {
    "name": "analyze_sentiment",
    "provider": "openai",
    "model": "gpt-4o-mini",
    "prompt": "Analyze sentiment of this customer review: {{review_text}}. Respond with JSON: {\"sentiment\": \"positive|neutral|negative\", \"score\": 0.0-1.0, \"key_phrases\": []}",
    "api_key": "{{secrets.openai_api_key}}",
    "temperature": 0.2
  }
}
```

### 4. Dynamic Decision Making

```json
{
  "type": "llm",
  "spec": {
    "name": "deployment_decision",
    "provider": "anthropic",
    "model": "claude-3-7-sonnet-20250219",
    "prompt": "Based on these test results: {{test_results}}, decide whether to deploy to production. Consider: test coverage={{coverage}}, failed tests={{failures}}, performance={{perf}}. Respond with JSON: {\"deploy\": boolean, \"reason\": string, \"recommendations\": []}",
    "api_key": "{{secrets.anthropic_api_key}}",
    "temperature": 0.1
  }
}
```

### 5. Code Generation

```json
{
  "type": "llm",
  "spec": {
    "name": "generate_sql",
    "provider": "openai",
    "model": "gpt-4o",
    "prompt": "Generate a SQL query to: {{user_request}}. Database schema: {{schema}}. Return only the SQL query.",
    "api_key": "{{secrets.openai_api_key}}",
    "system_prompt": "You are a SQL expert. Generate safe, optimized SQL queries. Always use parameterized queries to prevent SQL injection.",
    "temperature": 0.0
  }
}
```

### 6. Multi-Step Workflow with LLM

```json
[
  {
    "type": "http",
    "spec": {
      "name": "fetch_data",
      "url": "https://api.example.com/data",
      "method": "GET"
    }
  },
  {
    "type": "llm",
    "spec": {
      "name": "analyze_data",
      "provider": "anthropic",
      "model": "claude-3-7-sonnet-20250219",
      "prompt": "Analyze this data and identify trends: {{fetch_data.body}}. Return JSON with 'trends', 'insights', and 'recommendations'.",
      "api_key": "{{secrets.anthropic_api_key}}",
      "max_tokens": 2000
    }
  },
  {
    "type": "if",
    "spec": {
      "name": "check_recommendation",
      "condition": "{{analyze_data.output.recommendations | length}} > 0",
      "then": [
        {
          "type": "http",
          "spec": {
            "name": "send_alert",
            "url": "https://api.example.com/alerts",
            "method": "POST",
            "body": {
              "message": "New recommendations from LLM",
              "data": "{{analyze_data.output}}"
            }
          }
        }
      ]
    }
  }
]
```

## Complete Example

Full-featured LLM workflow with decision-making and downstream actions:

```json
{
  "type": "llm",
  "spec": {
    "name": "fraud_detection",
    "provider": "openai",
    "model": "gpt-4o",
    "prompt": "Analyze this transaction for fraud indicators. Transaction: amount={{transaction.amount}}, location={{transaction.location}}, time={{transaction.time}}, user_history={{user.history}}. Provide a fraud risk assessment.",
    "system_prompt": "You are a fraud detection specialist. Analyze transactions and provide structured risk assessments. Always respond with JSON containing: 'risk_level' (low|medium|high|critical), 'risk_score' (0.0-1.0), 'indicators' (array of detected patterns), 'recommendation' (approve|review|block).",
    "temperature": 0.0,
    "max_tokens": 500,
    "api_key": "{{secrets.openai_api_key}}"
  }
}
```

## Error Handling

If the LLM call fails, the error is stored in the response:

```json
{
  "provider": "openai",
  "model": "gpt-4",
  "prompt": "analyzed prompt",
  "response": "",
  "error": "API rate limit exceeded",
  "tokens_used": 0
}
```

The workflow will stop at the failed step unless error handling is implemented.

## Best Practices

1. **Use Structured Prompts**: Request JSON output for easy parsing
2. **Set Temperature Appropriately**: Use low (0-0.3) for deterministic tasks, higher (0.7-1.5) for creative tasks
3. **Secure API Keys**: Always use `{{secrets.key}}` pattern, never hardcode
4. **Validate Output**: Check `{{step.output}}` exists before using in subsequent steps
5. **Set Token Limits**: Prevent excessive costs and response times
6. **Custom System Prompts**: Tailor behavior for specific use cases
7. **Mock for Testing**: Use mock provider during development
8. **Handle Errors**: Plan for API failures and rate limits
9. **Keep Prompts Clear**: Specific, well-structured prompts get better results
10. **Chain Steps**: Combine LLM with other steps for powerful workflows

## Security Considerations

1. **API Key Management**: Store API keys in secrets, not in workflow definitions
2. **Prompt Injection**: Sanitize user inputs before including in prompts
3. **Data Privacy**: Be aware of data sent to external LLM providers
4. **Cost Control**: Set max_tokens limits to control costs
5. **Output Validation**: Validate LLM responses before using in critical operations
6. **Rate Limiting**: Plan for provider rate limits in high-volume scenarios

## Testing with Mock Provider

The mock provider allows testing workflows without LLM API calls:

```json
{
  "type": "llm",
  "spec": {
    "name": "test_llm",
    "provider": "mock",
    "model": "test-model",
    "prompt": "Give me structured JSON output"
  }
}
```

**Mock behavior:**

- Prompts containing "json" or "structured": Returns structured JSON
- Prompts containing "decision" or "approve": Returns decision JSON
- Prompts containing "error" or "fail": Returns error
- Other prompts: Returns simple text response

## Performance Considerations

- **Latency**: LLM API calls typically take 1-10 seconds depending on response length
- **Tokens**: Monitor token usage via `{{step.tokens_used}}` to control costs
- **Caching**: Consider caching LLM responses for identical prompts
- **Parallelization**: Use multiple LLM steps in parallel for independent tasks
- **Fallbacks**: Implement error handling and fallback logic for API failures

## Examples of Good vs. Bad Prompts

### ❌ Bad Prompt (Unstructured)

```
"What do you think about this data?"
```

### ✅ Good Prompt (Structured)

```
"Analyze this sales data: {{data}}. Return JSON with: 'total_sales' (number), 'top_product' (string), 'trend' (increasing|stable|decreasing), 'insights' (array of strings)."
```

### ❌ Bad Prompt (Vague)

```
"Check if this is okay"
```

### ✅ Good Prompt (Specific)

```
"Review this code change for: 1) Security vulnerabilities, 2) Performance issues, 3) Code style violations. Return JSON with 'issues' array containing {type, severity, line, description}."
```

## Troubleshooting

### LLM Returns Non-JSON Text

**Problem**: Response is not being parsed into `output` field.

**Solution**: Make your prompt more explicit about JSON format:

```json
"prompt": "Analyze {{data}}. IMPORTANT: Respond ONLY with valid JSON, no other text."
```

### API Authentication Errors

**Problem**: Provider returns 401 Unauthorized.

**Solution**: Verify API key is correct and passed via template:

```json
"api_key": "{{secrets.provider_key}}"
```

### Slow Response Times

**Problem**: LLM calls are taking too long.

**Solution**:

- Reduce `max_tokens`
- Use faster models (e.g., gpt-3.5-turbo instead of gpt-4)
- Simplify prompts to require shorter responses

### Inconsistent Outputs

**Problem**: LLM returns different structures each time.

**Solution**:

- Set `temperature` to 0.0 for maximum consistency
- Make output format requirements very explicit in prompt
- Provide examples of expected JSON structure
