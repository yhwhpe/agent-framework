package dispatcher

import (
	"context"
	"log"

	"github.com/yhwhpe/agent-framework/events"
)

// EventProcessor определяет интерфейс для обработки конкретных типов событий
type EventProcessor interface {
	HandleInit(ctx context.Context, event events.Event) error
	HandleContinue(ctx context.Context, event events.Event) error
}

// Dispatcher базовый диспетчер событий
type Dispatcher struct {
	processor EventProcessor
}

// New создает новый диспетчер
func New(processor EventProcessor) *Dispatcher {
	return &Dispatcher{
		processor: processor,
	}
}

// Handle маршрутизирует событие соответствующему обработчику
func (d *Dispatcher) Handle(ctx context.Context, ev events.Event) error {
	log.Printf("[DISPATCHER] 🎯 Starting event processing: eventId=%s, type=%s, chatID=%s",
		ev.EventID, ev.EventType, ev.ChatID)

	switch ev.EventType {
	case events.CliFlowInit:
		log.Printf("[DISPATCHER] 🚀 Routing to HandleInit: chatID=%s", ev.ChatID)
		return d.processor.HandleInit(ctx, ev)
	case events.CliFlowContinue:
		log.Printf("[DISPATCHER] ➡️  Routing to HandleContinue: chatID=%s", ev.ChatID)
		return d.processor.HandleContinue(ctx, ev)
	case events.CliFlowEnd:
		log.Printf("[DISPATCHER] 🏁 Event type CliFlowEnd - no processing needed: chatID=%s", ev.ChatID)
		return nil
	default:
		log.Printf("[DISPATCHER] ⚠️  Skipping unknown eventType=%s chatId=%s", ev.EventType, ev.ChatID)
		return nil
	}
}
