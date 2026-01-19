# Agent Framework Deployment Guide

## Publishing the Framework

### 1. Make Repository Public (if needed)
```bash
# Go to https://github.com/yhwhpe/agent-framework/settings
# Change visibility to Public
```

### 2. Tag Release
```bash
git tag v1.0.0
git push origin v1.0.0
```

### 3. Verify Publication
```bash
go mod download github.com/yhwhpe/agent-framework@v1.0.0
```

## Using the Framework in Projects

### For Public Repository:
```go
// go.mod
require github.com/yhwhpe/agent-framework v1.0.0
```

### For Private Repository:
```go
// go.mod
require github.com/yhwhpe/agent-framework v1.0.0

// For local development
replace github.com/yhwhpe/agent-framework => ../../framework
```

### Basic Usage Example:
```go
package main

import (
    "github.com/yhwhpe/agent-framework/app"
    "github.com/yhwhpe/agent-framework/config"
    "github.com/yhwhpe/agent-framework/events"
)

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

func main() {
    cfg, err := config.Load()
    if err != nil {
        panic(err)
    }

    handler := &MyAgent{}
    frameworkApp, err := app.New(cfg, handler)
    if err != nil {
        panic(err)
    }

    if err := frameworkApp.Run(); err != nil {
        panic(err)
    }
}
```

## Configuration

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

# MinIO for saga storage
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=my-agent-sagas
MINIO_USE_SSL=false
```

## Development Workflow

### Local Development:
1. Keep `replace github.com/yhwhpe/agent-framework => ../../framework` in go.mod
2. Make changes to local framework
3. Test with `go build ./...`
4. Push framework changes to GitHub
5. Create new version tag when ready

### Production Deployment:
1. Remove replace directive from go.mod
2. Use specific version: `github.com/yhwhpe/agent-framework v1.0.0`
3. Run `go mod tidy`

## Creating New Agents

1. Create new directory: `agents/new-agent/`
2. Copy structure from `agents/profile-builder/`
3. Implement your business logic in `internal/newagent/`
4. Update configuration and dependencies
5. Add to workspace: `go.work`

## Versioning Strategy

- Use semantic versioning: `v1.0.0`, `v1.1.0`, etc.
- Tag releases when framework API is stable
- Document breaking changes in release notes