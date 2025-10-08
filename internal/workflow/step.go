package workflow

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
)

func init() {
	// Register step types for gob encoding (required by DBOS)
	gob.Register(&HTTP{})
	gob.Register(&Action{})
	gob.Register(&If{})
	gob.Register(&Exec{})
	gob.Register(&Container{})
	gob.Register(&LLM{})
	// Register map types that may appear in step fields (e.g., HTTP.Body, Env)
	gob.Register(map[string]any{})
}

type StepType string

func (t StepType) String() string {
	return string(t)
}

const (
	StepTypeIf        StepType = "if"
	StepTypeAction    StepType = "action"
	StepTypeHTTP      StepType = "http"
	StepTypeExec      StepType = "exec"
	StepTypeContainer StepType = "container"
	StepTypeLLM       StepType = "llm"
)

type Executable interface {
	Execute(ctx context.Context, env Env) (Env, error)
	GetName() string
	Validate(env Env) error
}

type Step struct {
	Type StepType   `json:"type"`
	Spec Executable `json:"spec"`
}

func (s *Step) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type StepType        `json:"type"`
		Spec json.RawMessage `json:"spec"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Type = raw.Type
	switch raw.Type {
	case StepTypeIf:
		s.Spec = new(If)
	case StepTypeAction:
		s.Spec = new(Action)
	case StepTypeHTTP:
		s.Spec = new(HTTP)
	case StepTypeExec:
		s.Spec = new(Exec)
	case StepTypeContainer:
		s.Spec = new(Container)
	case StepTypeLLM:
		s.Spec = new(LLM)
	default:
		return fmt.Errorf("unknown step type: %s", raw.Type)
	}
	return json.Unmarshal(raw.Spec, s.Spec)
}
