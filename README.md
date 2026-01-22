# Agent Framework

A Go framework for building event-driven agents with RabbitMQ integration.

## Overview

Agent Framework provides reusable components for creating CLI flow-based agents that process events through RabbitMQ. The framework handles:

- Application lifecycle management
- RabbitMQ event consumption with retry logic
- Event routing and processing
- Configuration management
- Saga logging for distributed transactions

## Quick Start

```go
package main

import (
    "github.com/yhwhpe/agent-framework/app"
    "github.com/yhwhpe/agent-framework/config"
    "github.com/yhwhpe/agent-framework/events"
)

// Implement EventHandler interface
type MyAgent struct{}

func (a *MyAgent) Handle(ctx context.Context, event events.Event) error {
    switch event.EventType {
    case events.CliFlowInit:
        return a.handleInit(ctx, event)
    case events.CliFlowContinue:
        return a.handleContinue(ctx, event)
    }
    return nil
}

func (a *MyAgent) handleContinue(ctx context.Context, event events.Event) error {
    // Handle message-based continue events
    if event.Message != nil {
        return a.handleMessage(ctx, event)
    }

    // Handle action-based continue events
    if event.ActionType != "" {
        return a.handleAction(ctx, event)
    }

    return nil
}

func (a *MyAgent) handleAction(ctx context.Context, event events.Event) error {
    switch event.ActionType {
    case "upload_avatar":
        url := event.ActionData["url"].(string)
        return a.processAvatarUpload(ctx, event.ChatID, url)
    case "submit_form":
        formData := event.ActionData["form"].(map[string]any)
        return a.processFormSubmission(ctx, event.ChatID, formData)
    }
    return nil
}

func main() {
    cfg, _ := config.Load()
    handler := &MyAgent{}

    frameworkApp, _ := app.New(cfg, handler)
    frameworkApp.Run()
}
```

## Architecture

### Core Components

- **app**: Application lifecycle and HTTP health endpoints
- **config**: Configuration loading and validation
- **events**: CLI flow event definitions and structures
- **dispatcher**: Event routing framework with EventProcessor interface
- **rabbitmq**: RabbitMQ consumer with comprehensive logging and retry logic
- **saga**: Distributed transaction logging

### Interfaces

#### EventHandler
```go
type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

#### EventProcessor
```go
type EventProcessor interface {
    HandleInit(ctx context.Context, event Event) error
    HandleContinue(ctx context.Context, event Event) error
}
```

## Configuration

The framework provides core configuration. Agents should extend this with their specific settings:

```env
# Application
PORT=8080

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=cli.flow
RABBITMQ_QUEUE=my-agent
RABBITMQ_BINDINGS=cli.flow.my-agent
RABBITMQ_PREFETCH=1
RABBITMQ_MAX_RETRIES=3

# MinIO for saga storage (optional)
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=my-agent-sagas
MINIO_USE_SSL=false

# Agent-specific configurations should be added by the agent
# Examples:
# PRACTICE_URL=http://localhost:8080
# HASURA_URL=http://localhost:8080
# PROFILE_FORM_TYPE=user
```

## Event Types

- `CLI_FLOW_INIT`: Initialize agent conversation
- `CLI_FLOW_CONTINUE`: Process user input
- `CLI_FLOW_END`: Finalize conversation

## Logging

The framework provides comprehensive logging for:
- RabbitMQ message consumption and processing
- Event routing and handling
- Configuration loading
- Health check endpoints
- Saga event logging

## Health Checks

- `GET /healthz`: Basic health check
- `GET /readyz`: Readiness check including RabbitMQ connectivity

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License