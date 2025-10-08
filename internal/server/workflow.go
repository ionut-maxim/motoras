package server

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	workflowv1 "github.com/ionut-maxim/motoras/api/workflow/v1"
	workflowsvc "github.com/ionut-maxim/motoras/internal/workflow"
)

type WorkflowHandler struct {
	store   workflowsvc.Store
	service *workflowsvc.Service
}

func NewWorkflowHandler(store workflowsvc.Store, service *workflowsvc.Service) *WorkflowHandler {
	return &WorkflowHandler{
		store:   store,
		service: service,
	}
}

func (w *WorkflowHandler) Create(ctx context.Context, req *connect.Request[workflowv1.CreateRequest]) (*connect.Response[workflowv1.CreateResponse], error) {
	var workflow workflowsvc.Workflow
	if err := json.Unmarshal([]byte(req.Msg.Manifest), &workflow); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := w.store.Create(ctx, workflow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&workflowv1.CreateResponse{Id: id.String()})
	return res, nil
}

func (w *WorkflowHandler) Get(ctx context.Context, req *connect.Request[workflowv1.GetRequest]) (*connect.Response[workflowv1.GetResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow id"))
	}

	workflow, err := w.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, workflowsvc.ErrWorkflowNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Marshal workflow back to JSON manifest
	manifestJSON, err := json.Marshal(workflow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoWorkflow := &workflowv1.Workflow{
		Id:       workflow.ID.String(),
		Manifest: string(manifestJSON),
	}

	res := connect.NewResponse(&workflowv1.GetResponse{Workflow: protoWorkflow})
	return res, nil
}

func (w *WorkflowHandler) Update(ctx context.Context, req *connect.Request[workflowv1.UpdateRequest]) (*connect.Response[workflowv1.UpdateResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow id"))
	}

	var workflow workflowsvc.Workflow
	if err := json.Unmarshal([]byte(req.Msg.Manifest), &workflow); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Ensure the ID matches
	workflow.ID = id

	if err = w.store.Update(ctx, id, workflow); err != nil {
		if errors.Is(err, workflowsvc.ErrWorkflowNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&workflowv1.UpdateResponse{Id: id.String()})
	return res, nil
}

func (w *WorkflowHandler) Delete(ctx context.Context, req *connect.Request[workflowv1.DeleteRequest]) (*connect.Response[workflowv1.DeleteResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow id"))
	}

	if err = w.store.Delete(ctx, id); err != nil {
		if errors.Is(err, workflowsvc.ErrWorkflowNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&workflowv1.DeleteResponse{Id: id.String()})
	return res, nil
}

func (w *WorkflowHandler) List(ctx context.Context, req *connect.Request[workflowv1.ListRequest]) (*connect.Response[workflowv1.ListResponse], error) {
	workflows, err := w.store.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoWorkflows := make([]*workflowv1.Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		// Marshal workflow back to JSON manifest
		manifestJSON, err := json.Marshal(workflow)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		protoWorkflows = append(protoWorkflows, &workflowv1.Workflow{
			Id:       workflow.ID.String(),
			Manifest: string(manifestJSON),
		})
	}

	res := connect.NewResponse(&workflowv1.ListResponse{Workflows: protoWorkflows})
	return res, nil
}

func (w *WorkflowHandler) Start(ctx context.Context, req *connect.Request[workflowv1.StartRequest]) (*connect.Response[workflowv1.StartResponse], error) {
	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid workflow id"))
	}

	if err = w.service.StartWorkflow(ctx, id); err != nil {
		if errors.Is(err, workflowsvc.ErrWorkflowNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&workflowv1.StartResponse{
		Id:     id.String(),
		Status: "started",
	})
	return res, nil
}
