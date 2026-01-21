package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/yhwhpe/agent-framework/communicator"
	"github.com/yhwhpe/agent-framework/config"
	"github.com/yhwhpe/agent-framework/events"
	"github.com/yhwhpe/agent-framework/rabbitmq"
	"github.com/yhwhpe/agent-framework/saga"
)

// EventHandler определяет интерфейс для обработки событий
type EventHandler interface {
	Handle(ctx context.Context, event events.Event) error
}

// App представляет основное приложение агента
type App struct {
	cfg          *config.Config
	handler      EventHandler
	consumer     *rabbitmq.Consumer
	sagaLogger   saga.SagaLogger
	communicator *communicator.Client
	ready        atomic.Bool
}

// New создает новое приложение
func New(cfg *config.Config, handler EventHandler) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	// Initialize MinIO client for saga logger
	var minioClient *minio.Client
	if cfg.MinIO.Endpoint != "" && cfg.MinIO.AccessKey != "" && cfg.MinIO.SecretKey != "" {
		var err error
		minioClient, err = minio.New(cfg.MinIO.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
			Secure: cfg.MinIO.UseSSL,
		})
		if err != nil {
			log.Printf("⚠️ [FRAMEWORK] Failed to initialize MinIO client: %v", err)
			minioClient = nil
		} else {
			log.Printf("✅ [FRAMEWORK] MinIO client initialized")
		}
	} else {
		log.Printf("⚠️ [FRAMEWORK] MinIO not configured, saga logging disabled")
	}

	// Initialize Saga Logger
	sagaLogger := saga.NewMinIOSagaLogger(minioClient, cfg.MinIO.Bucket)
	log.Printf("✅ [FRAMEWORK] Saga logger initialized")

	// Initialize Communicator client
	var comm *communicator.Client
	if cfg.Communicator.BaseURL != "" {
		comm = communicator.New(cfg.Communicator.BaseURL, "")
		log.Printf("✅ [FRAMEWORK] Communicator client initialized")
	} else {
		log.Printf("⚠️ [FRAMEWORK] Communicator not configured")
	}

	consumer := rabbitmq.New(
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.Queue,
		cfg.RabbitMQ.Bindings,
		cfg.RabbitMQ.Prefetch,
		cfg.RabbitMQ.MaxRetries,
	)

	app := &App{
		cfg:          cfg,
		handler:      handler,
		consumer:     consumer,
		sagaLogger:   sagaLogger,
		communicator: comm,
	}

	return app, nil
}

// NewWithoutHandler создает приложение без handler (для получения saga logger)
func NewWithoutHandler(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	// Initialize MinIO client for saga logger
	var minioClient *minio.Client
	if cfg.MinIO.Endpoint != "" && cfg.MinIO.AccessKey != "" && cfg.MinIO.SecretKey != "" {
		var err error
		minioClient, err = minio.New(cfg.MinIO.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
			Secure: cfg.MinIO.UseSSL,
		})
		if err != nil {
			log.Printf("⚠️ [FRAMEWORK] Failed to initialize MinIO client: %v", err)
			minioClient = nil
		} else {
			log.Printf("✅ [FRAMEWORK] MinIO client initialized")
		}
	} else {
		log.Printf("⚠️ [FRAMEWORK] MinIO not configured, saga logging disabled")
	}

	// Initialize Saga Logger
	sagaLogger := saga.NewMinIOSagaLogger(minioClient, cfg.MinIO.Bucket)
	log.Printf("✅ [FRAMEWORK] Saga logger initialized")

	// Initialize Communicator client
	var comm *communicator.Client
	if cfg.Communicator.BaseURL != "" {
		comm = communicator.New(cfg.Communicator.BaseURL, "")
		log.Printf("✅ [FRAMEWORK] Communicator client initialized")
	} else {
		log.Printf("⚠️ [FRAMEWORK] Communicator not configured")
	}

	consumer := rabbitmq.New(
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.Queue,
		cfg.RabbitMQ.Bindings,
		cfg.RabbitMQ.Prefetch,
		cfg.RabbitMQ.MaxRetries,
	)

	app := &App{
		cfg:          cfg,
		handler:      nil, // Will be set later
		consumer:     consumer,
		sagaLogger:   sagaLogger,
		communicator: comm,
	}

	return app, nil
}

// SetHandler устанавливает handler для приложения
func (a *App) SetHandler(handler EventHandler) error {
	if handler == nil {
		return errors.New("handler cannot be nil")
	}
	a.handler = handler
	return nil
}

// Run запускает приложение
func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.ready.Store(true)
	defer a.consumer.Close()

	log.Printf("🚀 [FRAMEWORK] Starting agent with handler: %T", a.handler)

	return a.consumer.Consume(ctx, func(hctx context.Context, ev events.Event) error {
		ectx, cancel := context.WithTimeout(hctx, 30*time.Second)
		defer cancel()

		// Extract participant ID from event
		participantID := ev.ParticipantID

		// Set AI responding status to PROCESSING for continue events
		if ev.EventType == events.CliFlowContinue {
			if err := a.setAIRespondingStatus(ectx, ev.ChatID, participantID, "PROCESSING"); err != nil {
				log.Printf("[FRAMEWORK] ⚠️ Failed to set AI responding status to PROCESSING: %v", err)
				// Continue processing anyway
			}
		}

		// Process the event
		err := a.handler.Handle(ectx, ev)

		// Set AI responding status to DONE for continue events (success or failure)
		if ev.EventType == events.CliFlowContinue {
			status := "DONE"
			if err != nil {
				log.Printf("[FRAMEWORK] ⚠️ Event processing failed, but setting status to DONE: %v", err)
			}
			if setErr := a.setAIRespondingStatus(ectx, ev.ChatID, participantID, status); setErr != nil {
				log.Printf("[FRAMEWORK] ⚠️ Failed to set AI responding status to DONE: %v", setErr)
			}
		}

		return err
	})
}

// Router возвращает HTTP роутер для health checks
func (a *App) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", a.readyz)
	return mux
}

// GetSagaLogger возвращает логгер саги
func (a *App) GetSagaLogger() saga.SagaLogger {
	return a.sagaLogger
}

// setAIRespondingStatus отправляет статус AI responding в communicator
func (a *App) setAIRespondingStatus(ctx context.Context, chatID, participantID, status string) error {
	if a.communicator == nil {
		log.Printf("[FRAMEWORK] ⚠️ Communicator client not available, skipping AI responding status")
		return nil
	}

	// Convert status string to enum value
	var aiStatus string
	switch status {
	case "processing", "PROCESSING":
		aiStatus = "PROCESSING"
	case "done", "DONE":
		aiStatus = "DONE"
	default:
		return errors.New("invalid AI responding status: " + status)
	}

	log.Printf("[FRAMEWORK] 🤖 Setting AI responding status to %s for chat %s, participant %s", aiStatus, chatID, participantID)

	// Send status via GraphQL mutation
	query := `
		mutation SetAiRespondingStatus($input: SetAiRespondingStatusInput!) {
			setAiRespondingStatus(input: $input) {
				success
				error
			}
		}
	`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"chatId":        chatID,
			"participantId": participantID,
			"status":        aiStatus,
		},
	}

	var response struct {
		Data struct {
			SetAiRespondingStatus struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			} `json:"setAiRespondingStatus"`
		} `json:"data"`
	}

	err := a.communicator.GraphQL(ctx, query, variables, &response)
	if err != nil {
		return fmt.Errorf("failed to set AI responding status: %w", err)
	}

	if !response.Data.SetAiRespondingStatus.Success {
		return fmt.Errorf("failed to set AI responding status: %s", response.Data.SetAiRespondingStatus.Error)
	}

	log.Printf("[FRAMEWORK] ✅ Successfully set AI responding status to %s", aiStatus)
	return nil
}

func (a *App) readyz(w http.ResponseWriter, _ *http.Request) {
	if !a.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
		return
	}
	rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.consumer.Ping(rctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("rabbitmq not ready"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}
