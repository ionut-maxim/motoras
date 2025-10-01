package workflow

import (
	"context"
	"fmt"
)

type If struct {
	Name      string `json:"name,inline"`
	Condition string `json:"condition,inline"`
}

func (i *If) GetName() string {
	return i.Name
}

func (i *If) Validate(env Env) error {
	if i.Name == "" {
		return fmt.Errorf("missing name")
	}
	return env.new(i.Name)
}

func (i *If) Execute(ctx context.Context, env Env) (Env, error) {
	env[i.Name] = i.Condition
	return env, nil
}
