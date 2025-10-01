package main

import (
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"

	"github.com/ionut-maxim/motoras/internal/workflow"
)

type Engine struct {
	logger *slog.Logger
}

func main() {
	// Initialize a DBOS context
	ctx, err := dbos.NewDBOSContext(context.Background(), dbos.Config{
		DatabaseURL: os.Getenv("DBOS_SYSTEM_DATABASE_URL"),
		AppName:     "myapp",
		Logger:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	})
	if err != nil {
		panic(err)
	}

	wf := `
	{
	  "steps": [
		{
		  "type": "if",
		  "spec": {
			"name": "valid_name",
			"condition": "test > 1"
		  }
		},
		{
		  "type": "action",
		  "spec": {
			"name": "valid_name2",
			"inputs": {
			  "test": 1
			}
		  }
		}
	  ]
	}`

	// Register a workflow
	dbos.RegisterWorkflow(ctx, workflow.Run, dbos.WithWorkflowName("workflow"))
	gob.Register(map[string]interface{}{})

	// Launch DBOS
	err = ctx.Launch()
	if err != nil {
		panic(err)
	}
	defer ctx.Shutdown(3 * time.Second)

	// Run a durable workflow and get its result
	handle, err := dbos.RunWorkflow(ctx, workflow.Run, []byte(wf))
	if err != nil {
		fmt.Println(err)
	}
	res, err := handle.GetResult()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Workflow result:", res)
}
