package trigger

import "errors"

var (
	ErrNotImplemented        = errors.New("not implemented")
	ErrUnknownSubscriberType = errors.New("unknown subscriber type")
	ErrTriggerNotFound       = errors.New("trigger not found")
)
