package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP_Validate(t *testing.T) {
	tests := []struct {
		name    string
		step    *HTTP
		wantErr bool
	}{
		{
			name: "valid GET request",
			step: &HTTP{
				Name:   "get_test",
				URL:    "https://api.example.com",
				Method: "GET",
			},
			wantErr: false,
		},
		{
			name: "valid POST request",
			step: &HTTP{
				Name:   "post_test",
				URL:    "https://api.example.com",
				Method: "POST",
				Body:   map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			step: &HTTP{
				URL:    "https://api.example.com",
				Method: "GET",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			step: &HTTP{
				Name:   "test",
				Method: "GET",
			},
			wantErr: true,
		},
		{
			name: "defaults to GET when method is empty",
			step: &HTTP{
				Name: "default_method",
				URL:  "https://api.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := make(Env)
			err := tt.step.Validate(env)
			if (err != nil) != tt.wantErr {
				t.Errorf("HTTP.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTP_Execute_GET(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "success",
			"data":    []string{"item1", "item2"},
		})
	}))
	defer server.Close()

	step := &HTTP{
		Name:   "test_get",
		URL:    server.URL,
		Method: "GET",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("HTTP.Execute() error = %v", err)
	}

	// Check that response was stored in env
	response, ok := resultEnv[step.Name].(HTTPResponse)
	if !ok {
		t.Fatalf("Expected HTTPResponse in env, got %T", resultEnv[step.Name])
	}

	if response.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", response.StatusCode)
	}

	// Check that body was parsed as JSON
	body, ok := response.Body.(map[string]any)
	if !ok {
		t.Fatalf("Expected body to be parsed as map[string]any, got %T", response.Body)
	}

	if body["message"] != "success" {
		t.Errorf("Expected message='success', got %v", body["message"])
	}
}

func TestHTTP_Execute_POST_WithBody(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify Content-Type header
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type=application/json, got %s", contentType)
		}

		// Read and verify body
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if body["name"] != "test" {
			t.Errorf("Expected name='test', got %v", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      123,
			"created": true,
		})
	}))
	defer server.Close()

	step := &HTTP{
		Name:   "test_post",
		URL:    server.URL,
		Method: "POST",
		Body: map[string]any{
			"name":  "test",
			"value": 42,
		},
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("HTTP.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(HTTPResponse)
	if !ok {
		t.Fatalf("Expected HTTPResponse in env, got %T", resultEnv[step.Name])
	}

	if response.StatusCode != 201 {
		t.Errorf("Expected status code 201, got %d", response.StatusCode)
	}

	body, ok := response.Body.(map[string]any)
	if !ok {
		t.Fatalf("Expected body to be parsed as map[string]any, got %T", response.Body)
	}

	// JSON numbers are decoded as float64
	if body["id"] != float64(123) {
		t.Errorf("Expected id=123, got %v", body["id"])
	}
}

func TestHTTP_Execute_WithHeaders(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom headers
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("Expected X-Custom-Header=custom-value, got %s", r.Header.Get("X-Custom-Header"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	step := &HTTP{
		Name:   "test_headers",
		URL:    server.URL,
		Method: "GET",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	env := make(Env)
	ctx := context.Background()

	_, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("HTTP.Execute() error = %v", err)
	}
}

func TestHTTP_Execute_WithExpressions(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/123" {
			t.Errorf("Expected path /users/123, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":   123,
			"name": "John Doe",
		})
	}))
	defer server.Close()

	// Set up environment with variables
	env := make(Env)
	env["user_id"] = 123

	step := &HTTP{
		Name:   "get_user",
		URL:    server.URL + "/users/{{user_id}}",
		Method: "GET",
	}

	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("HTTP.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(HTTPResponse)
	if !ok {
		t.Fatalf("Expected HTTPResponse in env, got %T", resultEnv[step.Name])
	}

	if response.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", response.StatusCode)
	}
}

func TestHTTP_Execute_TextResponse(t *testing.T) {
	// Create a test server that returns plain text
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Plain text response"))
	}))
	defer server.Close()

	step := &HTTP{
		Name:   "test_text",
		URL:    server.URL,
		Method: "GET",
	}

	env := make(Env)
	ctx := context.Background()

	resultEnv, err := step.Execute(ctx, env)
	if err != nil {
		t.Fatalf("HTTP.Execute() error = %v", err)
	}

	response, ok := resultEnv[step.Name].(HTTPResponse)
	if !ok {
		t.Fatalf("Expected HTTPResponse in env, got %T", resultEnv[step.Name])
	}

	// For non-JSON responses, body should be string
	body, ok := response.Body.(string)
	if !ok {
		t.Fatalf("Expected body to be string for text response, got %T", response.Body)
	}

	if body != "Plain text response" {
		t.Errorf("Expected body='Plain text response', got %s", body)
	}
}

func TestHTTP_Execute_InvalidURL(t *testing.T) {
	step := &HTTP{
		Name:   "test_invalid",
		URL:    "http://this-domain-definitely-does-not-exist-12345.com",
		Method: "GET",
	}

	env := make(Env)
	ctx := context.Background()

	_, err := step.Execute(ctx, env)
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}
}
