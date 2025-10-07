# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Motoraș ("Little engine that could") is a workflow orchestration engine written in Go. It's designed to execute
workflows triggered by various events (e.g., scheduled tasks, webhooks, file system changes). The system uses PostgreSQL
for durable workflow execution via DBOS Transact.

## Development Setup

This project uses `mise` for tooling management. Required tools include:

- Go (latest)
- buf (Protocol Buffer tooling)
- protoc-gen-go and protoc-gen-connect-go (code generators)
- grpcurl (testing)

Install all tools:

```bash
mise install
```

Environment variables are loaded from `.env` file (see `.env.sample` for template). Configuration uses the `MOTO_`
prefix:

- `MOTO_POSTGRES_URL`: PostgreSQL connection string

## Common Commands

### Build and Run

```bash
# Build the main application
go build -o motoras ./cmd/motoras

# Run the application
go run ./cmd/motoras/main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run e2e tests only
go test ./e2e/...

# Run a single test
go test -run TestName ./path/to/package

# Run tests with verbose output
go test -v ./...
```

### Protocol Buffers

```bash
# Generate Go code from proto files
buf generate

# Lint proto files
buf lint

# Check for breaking changes
buf breaking --against '.git#branch=main'
```

## Architecture

### Core Components

**Application Container** (`internal/application`): Dependency injection container that wires up all services. It
creates the workflow and trigger services, establishes database connections, and starts the HTTP server. The container
pattern ensures proper lifecycle management and clean separation of concerns.

**Trigger Service** (`internal/trigger`): Event-driven system that manages trigger subscriptions. Each trigger runs in
its own worker goroutine with dynamic lifecycle management:

- Triggers can be added/updated/removed at runtime
- Workers automatically restart when trigger configuration changes
- Uses distributed locking to ensure each trigger runs on only one instance
- Subscribers decode trigger data and emit events to start workflows
- Currently includes a mock subscriber implementation for testing

Key files:

- `service.go`: Core service managing trigger workers and their lifecycle
- `subscriber.go`: Interface for trigger implementations (e.g., cron, webhook)
- `lock.go`: Distributed locking mechanism
- `store.go`: Trigger persistence interface with filesystem and mock implementations
- `METRICS.md`: OpenTelemetry metrics documentation for monitoring and load balancing

**Workflow Service** (`internal/workflow`): Orchestrates workflow execution using DBOS Transact for durability and
recoverability. Each workflow consists of multiple steps executed sequentially. DBOS ensures exactly-once execution
semantics and automatic recovery from failures.

Workflow structure:

- `Workflow`: Contains ID and array of Steps
- `Step`: Type-discriminated union with polymorphic Executable spec
- Step types: `if` (conditional), `action` (custom logic), `http` (HTTP requests)
- `Env`: Key-value context passed between steps for data flow

Key files:

- `service.go`: Workflow execution entry point
- `workflow.go`: Core workflow execution logic
- `step.go`: Step type system and polymorphic unmarshaling
- `step_*.go`: Individual step type implementations
- `env.go`: Environment/context for workflow execution

**HTTP Server** (`internal/server`): ConnectRPC-based API server supporting HTTP/2 (including unencrypted). Exposes
gRPC-style services with REST-friendly HTTP semantics. Uses interceptors for logging and validation.

Services:

- `TriggerService`: CRUD operations for triggers
- `WorkflowService`: CRUD operations for workflows

Server runs on port 8080 by default, configurable via listener injection for testing.

### Data Flow

1. Trigger Service loads all triggers from the store on startup
2. For each trigger, a worker goroutine is spawned with a Subscriber
3. Subscribers emit Events when conditions are met
4. Events flow to Application, which starts the associated Workflow
5. Workflow Service executes steps sequentially via DBOS
6. Each step has access to the workflow Env for passing data between steps

### Observability

**OpenTelemetry Metrics**: The trigger service exposes metrics for monitoring event processing and detecting back
pressure:

- `trigger.event_channel.size` (Gauge): Current number of events buffered in the event channel. High values indicate
  back pressure and can be used for load balancing decisions.
- `trigger.events.processed` (Counter): Total number of events processed. Useful for monitoring throughput and detecting
  anomalies.

See `internal/trigger/METRICS.md` for detailed documentation on metrics, configuration, and alerting recommendations.

### Testing Strategy

The codebase uses constructor injection and interface-based design for testability:

- Mock stores (trigger, workflow) for unit testing
- E2E tests use Unix domain sockets for fast IPC
- `TestMain` in e2e package sets up full application with test fixtures

## Project Structure

```
cmd/motoras/          - Application entry point
internal/
  application/        - Dependency injection container
  config/             - Environment-based configuration
  expression/         - Expression evaluation for workflows
  server/             - HTTP/ConnectRPC API handlers
  trigger/            - Trigger management and workers
    subscribers/      - Trigger subscriber implementations
      git/            - Git repository monitoring (commits, tags)
      mock/           - Mock subscriber for testing
  workflow/           - Workflow orchestration
proto/                - Protocol Buffer definitions
  trigger/v1/         - Trigger service API
  workflow/v1/        - Workflow service API
api/                  - Generated Go code from proto (do not edit)
e2e/                  - End-to-end tests
```

## Key Design Patterns

**Options Pattern**: Services accept functional options for flexible initialization (e.g., `WithStore`, `WithLogger`,
`WithListener`)

**Worker Pattern**: Trigger service spawns one goroutine per trigger, each with its own cancellation context and update
channel for dynamic reconfiguration

**Store Abstraction**: All persistence goes through Store interfaces. Currently implemented as mock stores for
development; real implementations will use PostgreSQL

**Type-Safe Polymorphism**: Workflow steps use Go's json.RawMessage for two-phase unmarshaling, enabling type-safe
polymorphic step definitions

## Development Notes

- The project is in active development (WIP state based on commit history)
- Real database persistence for triggers/workflows is not yet implemented
- Trigger expression parsing for conditional workflow starts is a TODO
- Log levels and handlers should eventually be configurable via application config
