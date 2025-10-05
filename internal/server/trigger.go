package server

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	triggerv1 "github.com/ionut-maxim/motoras/api/trigger/v1"
	triggerservice "github.com/ionut-maxim/motoras/internal/trigger"
)

type TriggerHandler struct {
	store triggerservice.Store
}

func NewTriggerHandler(store triggerservice.Store) *TriggerHandler {
	return &TriggerHandler{store: store}
}

func (t *TriggerHandler) Create(ctx context.Context, req *connect.Request[triggerv1.CreateRequest]) (*connect.Response[triggerv1.CreateResponse], error) {
	workflowID, err := uuid.Parse(req.Msg.Trigger.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var data any
	if err = json.Unmarshal(req.Msg.Trigger.Data.Value, &data); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	trigger := triggerservice.Trigger{
		ID:         id,
		Name:       req.Msg.Trigger.Name,
		Type:       req.Msg.Trigger.Type,
		WorkflowID: workflowID,
		Data:       nil,
	}

	if err = t.store.Add(ctx, trigger); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[triggerv1.CreateResponse]{}, nil
}

func (t *TriggerHandler) Update(ctx context.Context, req *connect.Request[triggerv1.UpdateRequest]) (*connect.Response[triggerv1.UpdateResponse], error) {
	return &connect.Response[triggerv1.UpdateResponse]{}, nil
}

func (t *TriggerHandler) List(ctx context.Context, req *connect.Request[triggerv1.ListRequest]) (*connect.Response[triggerv1.ListResponse], error) {
	return &connect.Response[triggerv1.ListResponse]{}, nil
}
