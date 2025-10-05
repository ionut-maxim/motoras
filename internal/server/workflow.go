package server

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"

	workflowv1 "github.com/ionut-maxim/motoras/api/workflow/v1"
	workflowsvc "github.com/ionut-maxim/motoras/internal/workflow"
)

type WorkflowHandler struct {
	store workflowsvc.Store
}

func NewWorkflowHandler(store workflowsvc.Store) *WorkflowHandler {
	return &WorkflowHandler{store}
}

func (w *WorkflowHandler) Create(ctx context.Context, req *connect.Request[workflowv1.CreateRequest]) (*connect.Response[workflowv1.CreateResponse], error) {
	var workflow workflowsvc.Workflow
	if err := json.Unmarshal([]byte(req.Msg.Manifest), &workflow); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := w.store.Create(ctx, workflow)
	if err != nil {
		return nil, err
	}

	res := connect.NewResponse(&workflowv1.CreateResponse{Id: id.String()})
	return res, nil
}
