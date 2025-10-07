package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"

	"github.com/ionut-maxim/motoras/api/trigger/v1/triggerv1connect"
	workflowv1 "github.com/ionut-maxim/motoras/api/workflow/v1"
	"github.com/ionut-maxim/motoras/api/workflow/v1/workflowv1connect"
	"github.com/ionut-maxim/motoras/internal/application"
	"github.com/ionut-maxim/motoras/internal/config"
)

var triggerClient triggerv1connect.TriggerServiceClient
var workflowClient workflowv1connect.WorkflowServiceClient

// TestMain sets up integration testing for our little engine. It uses unix domain socket for transport between client and server
func TestMain(m *testing.M) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	socketPath := fmt.Sprintf("%sm-%d.sock", os.TempDir(), time.Now().Nanosecond())
	defer os.Remove(socketPath)

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Error loading config", "error", err)
		return
	}

	app, err := application.New(cfg, logger, application.WithServerListener(lis))
	go func() {
		err = app.Start(ctx)
		if err != nil {
			return
		}
	}()

	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	triggerClient = triggerv1connect.NewTriggerServiceClient(httpClient, "http://localhost")
	workflowClient = workflowv1connect.NewWorkflowServiceClient(httpClient, "http://localhost")

	m.Run()
	time.Sleep(15 * time.Second)
}

func createWorkflow(ctx context.Context, t *testing.T, manifest string) *workflowv1.CreateResponse {
	req := &workflowv1.CreateRequest{Manifest: manifest}

	res, err := workflowClient.Create(ctx, connect.NewRequest(req))
	if err != nil {
		t.Error(err)
	}

	return res.Msg
}
