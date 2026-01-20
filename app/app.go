package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

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
	cfg        *config.Config
	handler    EventHandler
	consumer   *rabbitmq.Consumer
	sagaLogger saga.SagaLogger
	ready      atomic.Bool
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

	consumer := rabbitmq.New(
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.Queue,
		cfg.RabbitMQ.Bindings,
		cfg.RabbitMQ.Prefetch,
		cfg.RabbitMQ.MaxRetries,
	)

	app := &App{
		cfg:        cfg,
		handler:    handler,
		consumer:   consumer,
		sagaLogger: sagaLogger,
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

	consumer := rabbitmq.New(
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.Queue,
		cfg.RabbitMQ.Bindings,
		cfg.RabbitMQ.Prefetch,
		cfg.RabbitMQ.MaxRetries,
	)

	app := &App{
		cfg:        cfg,
		handler:    nil, // Will be set later
		consumer:   consumer,
		sagaLogger: sagaLogger,
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
		return a.handler.Handle(ectx, ev)
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
