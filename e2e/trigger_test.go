package e2e

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"

	triggerv1 "github.com/ionut-maxim/motoras/api/trigger/v1"
)

func Test_TriggerCreate(t *testing.T) {
	ctx := context.Background()

	workflowID := createWorkflow(ctx, t, `{"steps": []}`)

	req := &triggerv1.CreateRequest{Trigger: &triggerv1.Trigger{
		Id:         uuid.New().String(),
		Name:       "trigger-test",
		Type:       "mock",
		WorkflowId: workflowID.Id,
		Data:       &anypb.Any{Value: []byte(`{"input":"test"}`)},
	}}
	_, err := triggerClient.Create(ctx, connect.NewRequest(req))
	if err != nil {
		t.Error(err)
	}

}
