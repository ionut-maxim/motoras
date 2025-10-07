package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ionut-maxim/motoras/internal/expression"
)

type HTTP struct {
	Name    string            `json:"name"`
	URL     string            `json:"url" expression:"template"`
	Method  string            `json:"method" expression:"template"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // Timeout in seconds, default 30
}

type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       any               `json:"body"`
	RawBody    string            `json:"raw_body"`
}

func (h *HTTP) GetName() string {
	return h.Name
}

func (h *HTTP) Validate(env Env) error {
	if h.Name == "" {
		return errors.New("missing step name")
	}
	if h.URL == "" {
		return errors.New("missing URL")
	}
	if h.Method == "" {
		h.Method = "GET"
	}
	return env.new(h.Name)
}

func (h *HTTP) Execute(ctx context.Context, env Env) (Env, error) {
	// Evaluate expressions in the HTTP step using the current environment
	// Cast Env to map[string]any for expression evaluation
	evaluated, err := expression.Parse(ctx, h, map[string]any(env))
	if err != nil {
		return env, fmt.Errorf("failed to evaluate expressions: %w", err)
	}

	// Normalize method to uppercase
	method := strings.ToUpper(evaluated.Method)
	if method == "" {
		method = "GET"
	}

	// Evaluate body if it's a string (may contain templates), otherwise use as-is
	body := evaluated.Body
	if bodyStr, ok := body.(string); ok {
		type bodyValue struct {
			Value string `expression:"template"`
		}
		bv := bodyValue{Value: bodyStr}
		evaluatedBV, evalErr := expression.Parse(ctx, &bv, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate body: %w", evalErr)
		}
		body = evaluatedBV.Value
	}

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		var bodyBytes []byte
		switch v := body.(type) {
		case string:
			bodyBytes = []byte(v)
		case []byte:
			bodyBytes = v
		default:
			// Assume JSON-serializable object
			bodyBytes, err = json.Marshal(v)
			if err != nil {
				return env, fmt.Errorf("failed to marshal request body: %w", err)
			}
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, evaluated.URL, bodyReader)
	if err != nil {
		return env, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (manually evaluate templates in header values)
	for key, value := range evaluated.Headers {
		// Create a temporary struct to evaluate the header value
		type headerValue struct {
			Value string `expression:"template"`
		}
		hv := headerValue{Value: value}
		evaluatedHV, evalErr := expression.Parse(ctx, &hv, map[string]any(env))
		if evalErr != nil {
			return env, fmt.Errorf("failed to evaluate header %s: %w", key, evalErr)
		}
		req.Header.Set(key, evaluatedHV.Value)
	}

	// Set default Content-Type if body exists and no Content-Type is set
	if evaluated.Body != nil && req.Header.Get("Content-Type") == "" {
		if _, ok := evaluated.Body.(string); ok {
			req.Header.Set("Content-Type", "text/plain")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Configure timeout
	timeout := time.Duration(evaluated.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return env, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return env, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response headers
	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	// Try to parse body as JSON, fallback to string
	var parsedBody any
	if contentType := resp.Header.Get("Content-Type"); strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(respBody, &parsedBody); err != nil {
			// If JSON parsing fails, keep as string
			parsedBody = string(respBody)
		}
	} else {
		parsedBody = string(respBody)
	}

	// Store response in environment
	response := HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    respHeaders,
		Body:       parsedBody,
		RawBody:    string(respBody),
	}

	env[h.Name] = response

	return env, nil
}
