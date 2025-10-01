package workflow

import (
	"context"
	"fmt"
)

type Action struct {
	Name   string         `json:"name,inline"`
	Inputs map[string]any `json:"inputs,inline"`
}

func (a *Action) GetName() string {
	return a.Name
}

func (a *Action) Validate(env Env) error {
	if a.Name == "" {
		return fmt.Errorf("missing name")
	}
	return env.new(a.Name)
}

func (a *Action) Execute(ctx context.Context, env Env) (Env, error) {
	env[a.Name] = a.Inputs
	for name, input := range a.Inputs {
		fmt.Println(name+":", input)
	}
	return env, nil
}
