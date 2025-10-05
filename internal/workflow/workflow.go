package workflow

import (
	"context"
	"fmt"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/google/uuid"
)

type Workflow struct {
	ID    uuid.UUID `json:"id"`
	Steps []Step    `json:"steps"`
}

func run(ctx dbos.DBOSContext, wf Workflow) (Env, error) {
	var env = make(map[string]any)

	env, err := steps(ctx, wf.Steps, env)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func steps(ctx dbos.DBOSContext, steps []Step, env Env) (Env, error) {
	for _, step := range steps {
		var err error
		if err = step.Spec.Validate(env); err != nil {
			return nil, err
		}

		stepName := fmt.Sprintf("%s.%s", step.Type, step.Spec.GetName())
		env, err = dbos.RunAsStep(ctx, func(ctx context.Context) (Env, error) {
			var stepErr error
			env, stepErr = step.Spec.Execute(ctx, env)
			if stepErr != nil {
				return nil, stepErr
			}
			return env, nil
		}, dbos.WithStepName(stepName))
		if err != nil {
			return nil, err
		}

	}

	return env, nil
}
