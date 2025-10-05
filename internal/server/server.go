package server

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/ionut-maxim/motoras/api/trigger/v1/triggerv1connect"
	"github.com/ionut-maxim/motoras/api/workflow/v1/workflowv1connect"
	"github.com/ionut-maxim/motoras/internal/trigger"
	"github.com/ionut-maxim/motoras/internal/workflow"
)

func Start(logger *slog.Logger, workflowStore workflow.Store, triggerStore trigger.Store) error {
	mux := http.NewServeMux()

	logInterceptor := newLogInterceptor(logger)
	interceptors := connect.WithInterceptors(logInterceptor)

	tPath, tHandler := triggerv1connect.NewTriggerServiceHandler(NewTriggerHandler(triggerStore), interceptors)
	wfPath, wfHandler := workflowv1connect.NewWorkflowServiceHandler(NewWorkflowHandler(workflowStore), interceptors)
	mux.Handle(tPath, tHandler)
	mux.Handle(wfPath, wfHandler)
	p := new(http.Protocols)
	p.SetHTTP2(true)
	p.SetUnencryptedHTTP2(true)

	s := &http.Server{
		Addr:      "localhost:8080",
		Handler:   mux,
		Protocols: p,
	}
	return s.ListenAndServe()
}
