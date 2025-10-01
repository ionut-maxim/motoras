package workflow

import "errors"

type Env map[string]any

func (e Env) new(key string) error {
	if _, exists := e[key]; exists {
		return errors.New("duplicate step name: " + key)
	}
	e[key] = new(any)
	return nil
}
