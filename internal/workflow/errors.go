package workflow

import "errors"

var (
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowAlreadyExists = errors.New("workflow already exists")
	ErrUnableToGenerateID    = errors.New("unable to generate ID")
)
