package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration sourced from environment variables.
type Config struct {
	Port         string
	Communicator CommunicatorConfig
	RabbitMQ     RabbitMQConfig
	Practice     PracticeConfig
	Bot          BotConfig
	Axima        AximaConfig
	Mesa         MesaConfig
	Hasura       HasuraConfig
	LLM          LLMConfig
	MinIO        MinIOConfig
}

type CommunicatorConfig struct {
	BaseURL string
	Timeout time.Duration
}

type RabbitMQConfig struct {
	URL        string
	Exchange   string
	Queue      string
	Bindings   []string
	Prefetch   int
	MaxRetries int
}

type PracticeConfig struct {
	URL     string
	APIKey  string
	Timeout time.Duration
}

type BotConfig struct {
	Key              string
	EventStream      string
	StreamingEnabled bool
	ProfileFormType  string
}

type AximaConfig struct {
	URL     string
	Timeout time.Duration
}

type MesaConfig struct {
	URL     string
	Timeout time.Duration
}

type HasuraConfig struct {
	URL         string
	AdminSecret string
	Timeout     time.Duration
}

type LLMConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Timeout  time.Duration
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// Load reads environment variables and applies defaults.
func Load() (*Config, error) {
	port := getenvDefault("PORT", "8091")

	timeoutStr := getenvDefault("COMMUNICATOR_TIMEOUT", "10s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, errors.New("COMMUNICATOR_TIMEOUT is invalid: " + err.Error())
	}

	bindings := splitAndTrim(getenvDefault("RABBITMQ_BINDINGS", "cli.flow.profile-builder"))
	prefetch := intFromEnv("RABBITMQ_PREFETCH", 1)
	maxRetries := intFromEnv("RABBITMQ_MAX_RETRIES", 3)

	aluraTimeoutStr := getenvDefault("ALURA_TIMEOUT", "30s")
	aluraTimeout, err := time.ParseDuration(aluraTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("ALURA_TIMEOUT is invalid: %w", err)
	}

	practiceTimeoutStr := getenvDefault("PRACTICE_TIMEOUT", "30s")
	practiceTimeout, err := time.ParseDuration(practiceTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("PRACTICE_TIMEOUT is invalid: %w", err)
	}

	llmTimeoutStr := getenvDefault("LLM_TIMEOUT", "60s")
	llmTimeout, err := time.ParseDuration(llmTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("LLM_TIMEOUT is invalid: %w", err)
	}

	rabbitURL := getenvDefault("RABBITMQ_COMMUNICATOR_URL", os.Getenv("RABBITMQ_URL"))

	cfg := &Config{
		Port: port,
		Communicator: CommunicatorConfig{
			BaseURL: os.Getenv("COMMUNICATOR_URL"),
			Timeout: timeout,
		},
		RabbitMQ: RabbitMQConfig{
			URL:        rabbitURL,
			Exchange:   getenvDefault("RABBITMQ_EXCHANGE", "cli.flow"),
			Queue:      getenvDefault("RABBITMQ_QUEUE", "profile-builder"),
			Bindings:   bindings,
			Prefetch:   prefetch,
			MaxRetries: maxRetries,
		},
		Practice: PracticeConfig{
			URL:     os.Getenv("PRACTICE_URL"),
			APIKey:  os.Getenv("PRACTICE_INTERNAL_API_KEY"),
			Timeout: practiceTimeout,
		},
		Bot: BotConfig{
			Key:              getenvDefault("BOT_KEY", "profile-builder"),
			EventStream:      getenvDefault("BOT_EVENT_STREAM", "profile-builder-events"),
			StreamingEnabled: boolFromEnv("BOT_STREAMING_ENABLED", true),
			ProfileFormType:  getenvDefault("PROFILE_FORM_TYPE", "user"),
		},
		Axima: AximaConfig{
			URL:     os.Getenv("AXIMA_URL"),
			Timeout: 30 * time.Second, // Default timeout
		},
		Mesa: MesaConfig{
			URL:     os.Getenv("MESA_URL"),
			Timeout: 60 * time.Second, // Longer timeout for LLM operations
		},
		Hasura: HasuraConfig{
			URL:         os.Getenv("HASURA_URL"),
			AdminSecret: os.Getenv("HASURA_ADMIN_SECRET"),
			Timeout:     aluraTimeout,
		},
		LLM: LLMConfig{
			Provider: getenvDefault("LLM_PROVIDER", "deepseek"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			BaseURL:  getenvDefault("LLM_BASE_URL", "https://api.deepseek.com"),
			Model:    getenvDefault("LLM_MODEL", "deepseek-chat"),
			Timeout:  llmTimeout,
		},
		MinIO: MinIOConfig{
			Endpoint:  parseMinIOEndpoint(os.Getenv("MINIO_ENDPOINT")),
			AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET_KEY"),
			Bucket:    getenvDefault("MINIO_BUCKET", "profile-builder-sagas"),
			UseSSL:    boolFromEnv("MINIO_USE_SSL", false),
		},
	}

	if cfg.Communicator.BaseURL == "" {
		return nil, errors.New("COMMUNICATOR_URL is required")
	}
	if cfg.RabbitMQ.URL == "" {
		return nil, errors.New("RABBITMQ_URL is required")
	}
	if cfg.Hasura.URL == "" {
		return nil, errors.New("HASURA_URL is required")
	}
	if cfg.LLM.APIKey == "" {
		return nil, errors.New("LLM_API_KEY is required")
	}
	if cfg.MinIO.Endpoint == "" {
		return nil, errors.New("MINIO_ENDPOINT is required")
	}
	if cfg.MinIO.AccessKey == "" {
		return nil, errors.New("MINIO_ACCESS_KEY is required")
	}
	if cfg.MinIO.SecretKey == "" {
		return nil, errors.New("MINIO_SECRET_KEY is required")
	}

	return cfg, nil
}

func getenvDefault(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func splitAndTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func intFromEnv(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	if n, err := strconv.Atoi(val); err == nil {
		return n
	}
	return def
}

func boolFromEnv(key string, def bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}
	return def
}

// parseMinIOEndpoint parses MinIO endpoint, handling Railway-style templates and protocol prefixes
func parseMinIOEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}

	// Remove http:// or https:// prefix if present
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	// Handle Railway-style template variables ${{Bucket.MINIO_PRIVATE_HOST}}
	// If endpoint contains template syntax, extract the host:port part
	if strings.Contains(endpoint, "${{") && strings.Contains(endpoint, "}}") {
		// For Railway templates, we expect the format to be resolved by the platform
		// But if it's not resolved, we need to handle it gracefully
		// Return empty string to trigger validation error
		return ""
	}

	// Extract host:port from URL if it contains path
	// MinIO endpoint should be just host:port, not a full URL
	if strings.Contains(endpoint, "/") {
		// If it looks like a URL with path, extract just host:port
		parts := strings.SplitN(endpoint, "/", 2)
		endpoint = parts[0]
	}

	return endpoint
}
