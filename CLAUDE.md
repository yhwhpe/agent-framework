# agent-framework Library

Go фреймворк для создания event-driven агентов с RabbitMQ интеграцией.

**Language:** Go 1.24.3
**Type:** Library (не standalone service)

---

## 📍 Quick Links

- **Specification:** [docs/agent-framework.md](../docs/agent-framework.md)
- **Architecture:** [docs/architecture.md](../docs/architecture.md)
- **Workflows:** [docs/workflows.md](../docs/workflows.md)

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/yhwhpe/agent-framework
```

### Minimal Example

```go
package main

import (
    "context"
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
    case events.CliFlowEnd:
        return nil
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

---

## 📁 Framework Structure

```
agent-framework/
├── app/                            # Application lifecycle
│   ├── app.go                      # Main app struct
│   └── http.go                     # Health endpoints
├── config/                         # Configuration
│   └── config.go                   # Config loading
├── events/                         # Event definitions
│   ├── types.go                    # EventType enum
│   └── event.go                    # Event struct
├── dispatcher/                     # Event routing
│   ├── dispatcher.go               # Dispatcher
│   └── processor.go                # EventProcessor interface
├── rabbitmq/                       # RabbitMQ consumer
│   ├── consumer.go                 # Consumer implementation
│   └── retry.go                    # Retry logic
├── saga/                           # Saga logging
│   ├── logger.go                   # SagaLogger interface
│   └── minio.go                    # MinIO implementation
└── README.md                       # Library documentation
```

---

## 🔄 Core Interfaces

### EventHandler

Основной интерфейс для агентов.

```go
type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

**Implementation:**

```go
type MyAgent struct {
    communicator *communicator.Client
    llmClient    llm.Client
    sagaLogger   saga.SagaLogger
}

func (h *MyAgent) Handle(ctx context.Context, ev events.Event) error {
    switch ev.EventType {
    case events.CliFlowInit:
        return h.handleInit(ctx, ev)
    case events.CliFlowContinue:
        return h.handleContinue(ctx, ev)
    case events.CliFlowEnd:
        return nil
    }
    return nil
}
```

---

### EventProcessor

Альтернативный интерфейс с раздельными методами.

```go
type EventProcessor interface {
    HandleInit(ctx context.Context, event Event) error
    HandleContinue(ctx context.Context, event Event) error
}
```

**Implementation:**

```go
type MyProcessor struct{}

func (p *MyProcessor) HandleInit(ctx context.Context, ev events.Event) error {
    // Handle initialization
    return nil
}

func (p *MyProcessor) HandleContinue(ctx context.Context, ev events.Event) error {
    // Handle continuation
    if ev.Message != nil {
        return p.handleMessage(ctx, ev)
    }
    if ev.ActionType != "" {
        return p.handleAction(ctx, ev)
    }
    return nil
}
```

---

## 📊 Event Model

### Event Structure

```go
type Event struct {
    EventType     EventType         // CLI_FLOW_INIT | CLI_FLOW_CONTINUE | CLI_FLOW_END
    EventId       string            // UUID
    FlowId        string            // Session ID
    ChatId        string            // Chat ID
    ParticipantId string            // Participant ID
    AccountId     string            // Account ID
    Message       *string           // User message (for CONTINUE)
    ActionType    string            // Action type (for CONTINUE)
    ActionData    map[string]any    // Action data (for CONTINUE)
}

type EventType int

const (
    CliFlowInit EventType = iota
    CliFlowContinue
    CliFlowEnd
)
```

### Event Flow

```
CLI_FLOW_INIT
    ↓
Initialize conversation
    ↓
Send initial message
    ↓
CLI_FLOW_CONTINUE (message)
    ↓
Process user input
    ↓
Generate response
    ↓
Send message
    ↓
CLI_FLOW_CONTINUE (action)
    ↓
Process action (e.g., form submit)
    ↓
CLI_FLOW_END
    ↓
Finalize conversation
```

---

## 🔧 Configuration

### Environment Variables

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

# MinIO (для saga logging)
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=my-agent-sagas
MINIO_USE_SSL=false

# Agent-specific configurations
# (добавляйте свои переменные здесь)
```

### Loading Configuration

```go
import "github.com/yhwhpe/agent-framework/config"

type MyConfig struct {
    config.Config // Embed framework config
    LLMAPIKey     string `env:"LLM_API_KEY"`
    LLMModel      string `env:"LLM_MODEL" envDefault:"gpt-4"`
}

func LoadConfig() (*MyConfig, error) {
    cfg := &MyConfig{}
    if err := config.LoadInto(cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

---

## 📝 Saga Logging

### SagaLogger Interface

```go
type SagaLogger interface {
    LogEvent(ctx context.Context, accountID, chatID, eventType string, data map[string]any) error
    GetLogs(ctx context.Context, accountID, chatID string) ([]SagaEvent, error)
}
```

### Usage

```go
func (h *MyAgent) handleInit(ctx context.Context, ev events.Event) error {
    // Log phase start
    h.sagaLogger.LogEvent(ctx, ev.AccountId, ev.ChatId, "PHASE_STARTED", map[string]any{
        "phase": "init",
    })

    // Process initialization
    result := h.processInit(ctx, ev)

    // Log collected data
    h.sagaLogger.LogEvent(ctx, ev.AccountId, ev.ChatId, "COLLECTED_DATA", map[string]any{
        "field": "name",
        "value": result.Name,
        "confidence": 1.0,
    })

    return nil
}
```

### Saga Log Format (NDJSON)

```jsonl
{"eventType":"PHASE_STARTED","timestamp":1706102400000,"phase":"init"}
{"eventType":"LLM_REQUEST","timestamp":1706102401000,"requestId":"req_001","prompt":"..."}
{"eventType":"LLM_RESPONSE","timestamp":1706102404000,"requestId":"req_001","response":"..."}
{"eventType":"COLLECTED_DATA","timestamp":1706102404000,"field":"name","value":"John","confidence":1.0}
{"eventType":"PHASE_COMPLETED","timestamp":1706102405000,"phase":"init"}
```

---

## 🔗 RabbitMQ Integration

### Consumer Setup

Framework автоматически настраивает RabbitMQ consumer:

```
Exchange: cli.flow (topic, durable)
Queue: <RABBITMQ_QUEUE> (durable)
Binding: <RABBITMQ_BINDINGS> (routing key)

Prefetch: <RABBITMQ_PREFETCH> (default: 1)
```

### Retry Logic

Автоматический retry с exponential backoff:

```
Attempt 1: immediate
Attempt 2: +2 seconds
Attempt 3: +4 seconds
Max retries: <RABBITMQ_MAX_RETRIES> (default: 3)

After max retries: message rejected → dead letter queue (если настроен)
```

### Event Publishing

Агенты не публикуют события напрямую — используют communicator client:

```go
import "github.com/yhwhpe/communicator-client"

client := communicator.New("http://localhost:8084", "")

// Send message
client.SendAgentMessage(ctx, communicator.AgentMessageInput{
    ChatId: event.ChatId,
    Contents: []communicator.MessageContentItem{
        {
            Type:  "TEXT",
            Order: 1,
            Data: map[string]any{
                "text": "Hello from agent!",
            },
        },
    },
})
```

---

## 🔧 Health Endpoints

Framework предоставляет health endpoints:

```bash
# Basic health check
curl http://localhost:8080/healthz
# Response: {"status":"ok"}

# Readiness check (включает RabbitMQ connectivity)
curl http://localhost:8080/readyz
# Response: {"status":"ready","rabbitmq":"connected"}
```

---

## 🧮 Processing Pipeline

### λ-calculus View

```haskell
-- Event processing as function composition
processEvent :: Context -> Event -> IO Result

processEvent = λctx. λev. case ev.type of
    INIT     → processInit ctx ev
    CONTINUE → processContinue ctx ev
    END      → return ()

-- Continue processing splits on event payload
processContinue :: Context -> Event -> IO Result
processContinue ctx ev
    | isMessage ev  = processMessage ctx ev
    | isAction ev   = processAction ctx ev
    | otherwise     = return emptyResult
```

### Processing Stages

```
Input Event
    ↓
Dispatcher → Route by EventType
    ↓
EventHandler.Handle() or EventProcessor.HandleInit/HandleContinue()
    ↓
Business Logic
    ↓
Communicator Client → Send Response
    ↓
Saga Logger → Log Events
    ↓
Complete
```

---

## 📦 Example Agents

### 1. profile-builder

**Location:** `agents/profile-builder/`

**Purpose:** Собирает профиль пользователя через интерактивный опрос

**Processing:**

```go
func (h *Handler) Handle(ctx context.Context, ev events.Event) error {
    switch ev.EventType {
    case events.CliFlowInit:
        return h.handleInit(ctx, ev)  // Send first question
    case events.CliFlowContinue:
        if ev.Message != nil {
            return h.handleMessage(ctx, ev)  // Process answer
        }
        if ev.ActionType != "" {
            return h.handleAction(ctx, ev)  // Process action (e.g., avatar upload)
        }
    case events.CliFlowEnd:
        return h.finalizeProfile(ctx, ev)  // Save profile
    }
    return nil
}
```

---

### 2. alura

**Location:** `agents/alura/`

**Purpose:** AI assistant для пользователей и специалистов

**Processing:**

```go
func (h *Handler) Handle(ctx context.Context, ev events.Event) error {
    switch ev.EventType {
    case events.CliFlowInit:
        return h.sendGreeting(ctx, ev)
    case events.CliFlowContinue:
        if ev.Message != nil {
            return h.processUserMessage(ctx, ev)
        }
    }
    return nil
}

func (h *Handler) processUserMessage(ctx context.Context, ev events.Event) error {
    // Generate LLM response
    response := h.llmClient.Generate(ctx, *ev.Message)

    // Send response
    h.communicator.SendAgentMessage(ctx, communicator.AgentMessageInput{
        ChatId: ev.ChatId,
        Contents: []communicator.MessageContentItem{
            {Type: "TEXT", Order: 1, Data: map[string]any{"text": response}},
        },
    })

    return nil
}
```

---

### 3. mesa

**Location:** `agents/mesa/`

**Purpose:** Анамнез и классификация для генерации milestone карт

**Processing:**

```go
func (h *Handler) Handle(ctx context.Context, ev events.Event) error {
    switch ev.EventType {
    case events.CliFlowInit:
        return h.startInterview(ctx, ev)
    case events.CliFlowContinue:
        if ev.Message != nil {
            return h.processInterviewAnswer(ctx, ev)
        }
    case events.CliFlowEnd:
        return h.generateMilestoneMap(ctx, ev)
    }
    return nil
}
```

---

## 🐛 Common Issues

### 1. RabbitMQ connection fails

**Problem:** Agent can't connect to RabbitMQ

**Check:**

```bash
# Is RabbitMQ running?
docker ps | grep rabbitmq

# Check connection
rabbitmqctl status

# Test connection from agent
curl http://localhost:15672/api/overview
```

**Solution:**

```bash
# Restart RabbitMQ
docker-compose restart rabbitmq

# Check .env
echo $RABBITMQ_URL
```

---

### 2. Events not received

**Problem:** Agent doesn't receive events

**Check:**

1. Queue exists: http://localhost:15672/#/queues
2. Binding correct: Check routing key matches
3. Consumer connected: Check connections in RabbitMQ UI

**Solution:**

```bash
# Manually bind queue to exchange
rabbitmqadmin declare binding \
    source=cli.flow \
    destination=my-agent \
    routing_key=cli.flow.my-agent
```

---

### 3. Saga logs not saved

**Problem:** Saga logs not appearing in MinIO

**Check:**

```bash
# Is MinIO running?
docker ps | grep minio

# Check bucket exists
mc ls myminio/my-agent-sagas

# Check credentials
echo $MINIO_ACCESS_KEY $MINIO_SECRET_KEY
```

---

## 📚 Additional Documentation

- [CLAUDE.md](../CLAUDE.md) — Root entry point
- [docs/agent-framework.md](../docs/agent-framework.md) — Complete specification
- [docs/architecture.md](../docs/architecture.md) — System architecture
- [docs/workflows.md](../docs/workflows.md) — Development workflows

---

## 🔗 Related Libraries

- **communicator-client** — Go client for communicator service
- **llm-unified-client** — Unified LLM client (OpenAI, Mistral, DeepSeek)
- **agent-hub-adapter-client** — Client for LLM routing

---

**Версия:** 2.0
**Обновлено:** 2026-01-24
