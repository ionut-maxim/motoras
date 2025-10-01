package workflow

import (
	"context"
	"errors"
)

type HTTP struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Method string `json:"method"`
}

func (h *HTTP) GetName() string {
	return h.Name
}

func (h *HTTP) Validate(env Env) error {
	if h.Name == "" {
		return errors.New("missing step name")
	}
	return env.new(h.Name)
}

func (h *HTTP) Execute(ctx context.Context, env Env) (Env, error) {
	env[h.Name] = h.URL
	return env, nil
}
