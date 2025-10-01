package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

type Workflow struct {
	Steps []Step `json:"steps"`
}

func Run(ctx dbos.DBOSContext, payload []byte) (Env, error) {
	var env = make(map[string]any)

	var wf Workflow
	if err := json.Unmarshal(payload, &wf); err != nil {
		return nil, err
	}

	env, err := processSteps(ctx, wf.Steps, env)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func processSteps(ctx dbos.DBOSContext, steps []Step, env Env) (Env, error) {
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
