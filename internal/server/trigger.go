package server

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"

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
	if req.Msg.Trigger == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trigger is required"))
	}

	var id uuid.UUID
	var err error

	// If ID is provided, use it; otherwise generate a new one
	if req.Msg.Trigger.Id != "" {
		id, err = uuid.Parse(req.Msg.Trigger.Id)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid trigger id"))
		}
	} else {
		id, err = uuid.NewV7()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	workflowID, err := uuid.Parse(req.Msg.Trigger.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow_id"))
	}

	var data any
	if err = json.Unmarshal(req.Msg.Trigger.Data.Value, &data); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	trigger := triggerservice.Trigger{
		ID:         id,
		Name:       req.Msg.Trigger.Name,
		Type:       req.Msg.Trigger.Type,
		WorkflowID: workflowID,
		Data:       data,
	}

	if err = t.store.Add(ctx, trigger); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&triggerv1.CreateResponse{Id: id.String()})
	return res, nil
}

func (t *TriggerHandler) Get(ctx context.Context, req *connect.Request[triggerv1.GetRequest]) (*connect.Response[triggerv1.GetResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid trigger id"))
	}

	trigger, err := t.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, triggerservice.ErrTriggerNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Marshal data back to JSON
	dataJSON, err := json.Marshal(trigger.Data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoTrigger := &triggerv1.Trigger{
		Id:         trigger.ID.String(),
		Name:       trigger.Name,
		Type:       trigger.Type,
		WorkflowId: trigger.WorkflowID.String(),
		Data:       &anypb.Any{Value: dataJSON},
	}

	res := connect.NewResponse(&triggerv1.GetResponse{Trigger: protoTrigger})
	return res, nil
}

func (t *TriggerHandler) Update(ctx context.Context, req *connect.Request[triggerv1.UpdateRequest]) (*connect.Response[triggerv1.UpdateResponse], error) {
	if req.Msg.Trigger == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trigger is required"))
	}

	id, err := uuid.Parse(req.Msg.Trigger.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid trigger id"))
	}

	workflowID, err := uuid.Parse(req.Msg.Trigger.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow_id"))
	}

	var data any
	if err = json.Unmarshal(req.Msg.Trigger.Data.Value, &data); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	trigger := triggerservice.Trigger{
		ID:         id,
		Name:       req.Msg.Trigger.Name,
		Type:       req.Msg.Trigger.Type,
		WorkflowID: workflowID,
		Data:       data,
	}

	if err = t.store.Update(ctx, trigger); err != nil {
		if errors.Is(err, triggerservice.ErrTriggerNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&triggerv1.UpdateResponse{Id: id.String()})
	return res, nil
}

func (t *TriggerHandler) Delete(ctx context.Context, req *connect.Request[triggerv1.DeleteRequest]) (*connect.Response[triggerv1.DeleteResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid trigger id"))
	}

	if err = t.store.Delete(ctx, id); err != nil {
		if errors.Is(err, triggerservice.ErrTriggerNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&triggerv1.DeleteResponse{Id: id.String()})
	return res, nil
}

func (t *TriggerHandler) List(ctx context.Context, req *connect.Request[triggerv1.ListRequest]) (*connect.Response[triggerv1.ListResponse], error) {
	triggers, err := t.store.All(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoTriggers := make([]*triggerv1.Trigger, 0, len(triggers))
	for _, trigger := range triggers {
		// Marshal data back to JSON
		dataJSON, err := json.Marshal(trigger.Data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		protoTriggers = append(protoTriggers, &triggerv1.Trigger{
			Id:         trigger.ID.String(),
			Name:       trigger.Name,
			Type:       trigger.Type,
			WorkflowId: trigger.WorkflowID.String(),
			Data:       &anypb.Any{Value: dataJSON},
		})
	}

	res := connect.NewResponse(&triggerv1.ListResponse{Triggers: protoTriggers})
	return res, nil
}
