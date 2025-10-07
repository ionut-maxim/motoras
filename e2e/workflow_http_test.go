package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"

	triggerv1 "github.com/ionut-maxim/motoras/api/trigger/v1"
)

// Test_WorkflowWithHTTPStep tests the complete flow:
// 1. Creates a test HTTP server that writes a file when called
// 2. Creates a workflow with an HTTP step that calls this server
// 3. Creates a trigger that fires every 5 seconds
// 4. Waits for the file to be created (proving the entire flow works)
func Test_WorkflowWithHTTPStep(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for test files
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("motoras-test-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	testFilePath := filepath.Join(testDir, "workflow-executed.txt")

	// Create a test HTTP server that writes a file when called
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Test server received request: %s %s", r.Method, r.URL.Path)

		// Verify the request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Write a file to prove the workflow executed
		timestamp := time.Now().Format(time.RFC3339)
		content := fmt.Sprintf("Workflow executed at %s\n", timestamp)
		if err := os.WriteFile(testFilePath, []byte(content), 0644); err != nil {
			t.Errorf("Failed to write test file: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		t.Logf("Test server wrote file: %s", testFilePath)

		// Return a success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "success",
			"timestamp": timestamp,
			"message":   "File written successfully",
		})
	}))
	defer testServer.Close()

	t.Logf("Test HTTP server started at: %s", testServer.URL)

	// Create a workflow with an HTTP step that calls our test server
	workflowManifest := fmt.Sprintf(`{
		"steps": [
			{
				"type": "http",
				"spec": {
					"name": "call_test_server",
					"url": "%s/write-file",
					"method": "POST",
					"headers": {
						"Content-Type": "application/json",
						"X-Test-Header": "e2e-test"
					},
					"body": {
						"test": "data",
						"source": "motoras-e2e-test"
					}
				}
			}
		]
	}`, testServer.URL)

	t.Logf("Creating workflow with HTTP step")
	workflow := createWorkflow(ctx, t, workflowManifest)
	if workflow == nil || workflow.Id == "" {
		t.Fatal("Failed to create workflow")
	}
	t.Logf("Workflow created with ID: %s", workflow.Id)

	// Create a trigger that fires every 5 seconds
	triggerData := map[string]string{
		"input": "http-test-trigger",
	}
	triggerDataJSON, err := json.Marshal(triggerData)
	if err != nil {
		t.Fatalf("Failed to marshal trigger data: %v", err)
	}

	triggerReq := &triggerv1.CreateRequest{
		Trigger: &triggerv1.Trigger{
			Id:         uuid.New().String(),
			Name:       "http-workflow-trigger",
			Type:       "mock",
			WorkflowId: workflow.Id,
			Data:       &anypb.Any{Value: triggerDataJSON},
		},
	}

	t.Logf("Creating trigger for workflow %s", workflow.Id)
	_, err = triggerClient.Create(ctx, connect.NewRequest(triggerReq))
	if err != nil {
		t.Fatalf("Failed to create trigger: %v", err)
	}
	t.Logf("Trigger created successfully")

	// Wait for the file to be created (with timeout)
	// The mock trigger fires every 5 seconds, so we should see the file within ~10 seconds
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	t.Logf("Waiting for file to be created: %s", testFilePath)

	for {
		select {
		case <-ticker.C:
			// Check if the file exists
			if _, err := os.Stat(testFilePath); err == nil {
				// File exists! Read its content
				content, readErr := os.ReadFile(testFilePath)
				if readErr != nil {
					t.Fatalf("File exists but failed to read: %v", readErr)
				}
				t.Logf("SUCCESS! File created with content: %s", string(content))
				return
			} else if !os.IsNotExist(err) {
				t.Fatalf("Error checking file: %v", err)
			}
			// File doesn't exist yet, continue waiting

		case <-timeout:
			t.Fatal("Timeout waiting for workflow to execute and create file")
		}
	}
}
