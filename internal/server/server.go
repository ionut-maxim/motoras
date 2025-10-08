package server

import (
	"log/slog"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"github.com/ionut-maxim/motoras/api/trigger/v1/triggerv1connect"
	"github.com/ionut-maxim/motoras/api/workflow/v1/workflowv1connect"
	"github.com/ionut-maxim/motoras/internal/trigger"
	"github.com/ionut-maxim/motoras/internal/workflow"
)

type Server struct {
	logger          *slog.Logger
	workflowStore   workflow.Store
	workflowService *workflow.Service
	triggerStore    trigger.Store

	listener net.Listener
}

type Option func(*Server)

func WithListener(listener net.Listener) Option {
	return func(s *Server) {
		s.listener = listener
	}
}

func New(logger *slog.Logger, workflowStore workflow.Store, workflowService *workflow.Service, triggerStore trigger.Store, options ...Option) *Server {
	s := &Server{
		logger:          logger,
		workflowStore:   workflowStore,
		workflowService: workflowService,
		triggerStore:    triggerStore,
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Server) Start() error {
	var err error

	if s.listener == nil {
		s.listener, err = net.Listen("tcp", ":8080")
		if err != nil {
			return err
		}
	}

	logInterceptor := newLogInterceptor(s.logger)
	validateInterceptor := validate.NewInterceptor()
	interceptors := connect.WithInterceptors(logInterceptor, validateInterceptor)

	tPath, tHandler := triggerv1connect.NewTriggerServiceHandler(NewTriggerHandler(s.triggerStore), interceptors)
	wfPath, wfHandler := workflowv1connect.NewWorkflowServiceHandler(NewWorkflowHandler(s.workflowStore, s.workflowService), interceptors)

	mux := http.NewServeMux()
	mux.Handle(tPath, tHandler)
	mux.Handle(wfPath, wfHandler)

	p := new(http.Protocols)
	p.SetHTTP2(true)
	p.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Handler:   mux,
		Protocols: p,
	}
	s.logger.Info("Attempting to listen", "address", s.listener.Addr().String())
	return srv.Serve(s.listener)
}
