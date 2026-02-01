package saga

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/minio/minio-go/v7"
)

// SagaLogger интерфейс для логирования событий саги
type SagaLogger interface {
	LogEvent(ctx context.Context, accountID, chatID string, event EventUnion) error
	GetEvents(ctx context.Context, accountID, chatID string, eventTypes []string) ([]EventUnion, error)
	GetLastEvent(ctx context.Context, accountID, chatID string, eventType string) (EventUnion, error)
	GetPhaseEvents(ctx context.Context, accountID, chatID, phase string) ([]EventUnion, error)
	GetCollectedData(ctx context.Context, accountID, chatID string) (map[string]interface{}, error)
}

// MinIOSagaLogger реализация SagaLogger с использованием MinIO
type MinIOSagaLogger struct {
	minioClient *minio.Client
	bucketName  string
}

// NewMinIOSagaLogger создает новый экземпляр MinIOSagaLogger
func NewMinIOSagaLogger(minioClient *minio.Client, bucketName string) *MinIOSagaLogger {
	return &MinIOSagaLogger{
		minioClient: minioClient,
		bucketName:  bucketName,
	}
}

// getObjectName возвращает путь к файлу саги в MinIO
func (l *MinIOSagaLogger) getObjectName(accountID, chatID string) string {
	return fmt.Sprintf("%s/%s/saga.log", accountID, chatID)
}

// LogEvent логирует событие в сагу
func (l *MinIOSagaLogger) LogEvent(ctx context.Context, accountID, chatID string, event EventUnion) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	objectName := l.getObjectName(accountID, chatID)

	// Получить существующий контент файла
	existingContent, err := l.getExistingContent(ctx, objectName)
	if err != nil {
		log.Printf("[SAGA] Failed to get existing content for %s: %v", objectName, err)
		// Если файл не существует, начнем с пустого контента
		existingContent = []byte{}
	}

	// Добавить новое событие
	newContent := append(existingContent, eventJSON...)
	newContent = append(newContent, '\n')

	// Загрузить обновленный контент
	return l.uploadContent(ctx, objectName, newContent)
}

// getExistingContent получает существующий контент файла саги
func (l *MinIOSagaLogger) getExistingContent(ctx context.Context, objectName string) ([]byte, error) {
	obj, err := l.minioClient.GetObject(ctx, l.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		// Если объект не существует, возвращаем пустой контент
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return []byte{}, nil
		}
		return nil, fmt.Errorf("failed to get object %s: %w", objectName, err)
	}
	defer obj.Close()

	content, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return content, nil
}

// uploadContent загружает контент в MinIO
func (l *MinIOSagaLogger) uploadContent(ctx context.Context, objectName string, content []byte) error {
	reader := bytes.NewReader(content)

	_, err := l.minioClient.PutObject(ctx, l.bucketName, objectName, reader, int64(len(content)),
		minio.PutObjectOptions{
			ContentType: "application/x-ndjson",
		})

	if err != nil {
		return fmt.Errorf("failed to upload saga content to %s: %w", objectName, err)
	}

	log.Printf("[SAGA] Successfully logged event to %s (%d bytes)", objectName, len(content))
	return nil
}

// GetEvents получает все события указанных типов
func (l *MinIOSagaLogger) GetEvents(ctx context.Context, accountID, chatID string, eventTypes []string) ([]EventUnion, error) {
	objectName := l.getObjectName(accountID, chatID)

	content, err := l.getExistingContent(ctx, objectName)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []EventUnion{}, nil
	}

	return l.parseEvents(content, eventTypes)
}

// GetLastEvent получает последнее событие указанного типа
func (l *MinIOSagaLogger) GetLastEvent(ctx context.Context, accountID, chatID string, eventType string) (EventUnion, error) {
	events, err := l.GetEvents(ctx, accountID, chatID, []string{eventType})
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("no events of type %s found", eventType)
	}

	return events[len(events)-1], nil
}

// GetPhaseEvents получает все события для указанной фазы
func (l *MinIOSagaLogger) GetPhaseEvents(ctx context.Context, accountID, chatID, phase string) ([]EventUnion, error) {
	allEvents, err := l.GetEvents(ctx, accountID, chatID, []string{})
	if err != nil {
		return nil, err
	}

	var phaseEvents []EventUnion
	for _, event := range allEvents {
		// Проверяем, есть ли поле phase в событии
		switch e := event.(type) {
		case *PhaseStartedEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *PhaseCompletedEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *LLMRequestEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *LLMResponseEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *CollectedDataEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *MesaAnalysisEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		case *AximaMilestoneEvent:
			if e.Phase == phase {
				phaseEvents = append(phaseEvents, e)
			}
		}
	}

	return phaseEvents, nil
}

// GetCollectedData получает все собранные данные профиля
func (l *MinIOSagaLogger) GetCollectedData(ctx context.Context, accountID, chatID string) (map[string]interface{}, error) {
	events, err := l.GetEvents(ctx, accountID, chatID, []string{"COLLECTED_DATA"})
	if err != nil {
		return nil, err
	}

	collectedData := make(map[string]interface{})

	for _, event := range events {
		if collectedEvent, ok := event.(*CollectedDataEvent); ok {
			collectedData[collectedEvent.Field] = collectedEvent.Value
		}
	}

	return collectedData, nil
}

// parseEvents парсит JSON Lines контент и фильтрует по типам событий
func (l *MinIOSagaLogger) parseEvents(content []byte, eventTypes []string) ([]EventUnion, error) {
	var events []EventUnion

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		event, err := l.parseEvent(line)
		if err != nil {
			log.Printf("[SAGA] Failed to parse event line: %v", err)
			continue // Пропускаем поврежденные строки
		}

		// Фильтруем по типам событий, если указаны
		if len(eventTypes) == 0 || contains(eventTypes, event.GetEventType()) {
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan saga content: %w", err)
	}

	return events, nil
}

// parseEvent парсит одно событие из JSON
func (l *MinIOSagaLogger) parseEvent(jsonLine string) (EventUnion, error) {
	// Сначала определяем тип события
	var baseEvent struct {
		EventType string `json:"eventType"`
	}

	if err := json.Unmarshal([]byte(jsonLine), &baseEvent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal base event: %w", err)
	}

	var event EventUnion

	switch baseEvent.EventType {
	case "PHASE_STARTED":
		event = &PhaseStartedEvent{}
	case "PHASE_COMPLETED":
		event = &PhaseCompletedEvent{}
	case "LLM_REQUEST":
		event = &LLMRequestEvent{}
	case "LLM_RESPONSE":
		event = &LLMResponseEvent{}
	case "MESSAGE_ADDED":
		event = &MessageAddedEvent{}
	case "LLM_QUESTION":
		event = &MessageAddedEvent{}
	case "LLM_RAW_RESPONSE":
		event = &MessageAddedEvent{}
	case "COLLECTED_DATA":
		event = &CollectedDataEvent{}
	case "MESA_ANALYSIS":
		event = &MesaAnalysisEvent{}
	case "AXIMA_MILESTONE":
		event = &AximaMilestoneEvent{}
	default:
		return nil, fmt.Errorf("unknown event type: %s", baseEvent.EventType)
	}

	if err := json.Unmarshal([]byte(jsonLine), event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event of type %s: %w", baseEvent.EventType, err)
	}

	return event, nil
}

// contains проверяет, содержит ли слайс указанную строку
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// BatchLogEvents логирует несколько событий атомарно
func (l *MinIOSagaLogger) BatchLogEvents(ctx context.Context, accountID, chatID string, events []EventUnion) error {
	if len(events) == 0 {
		return nil
	}

	objectName := l.getObjectName(accountID, chatID)

	// Получить существующий контент
	existingContent, err := l.getExistingContent(ctx, objectName)
	if err != nil {
		log.Printf("[SAGA] Failed to get existing content for batch log: %v", err)
		existingContent = []byte{}
	}

	// Добавить все новые события
	newContent := existingContent
	for _, event := range events {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event in batch: %w", err)
		}
		newContent = append(newContent, eventJSON...)
		newContent = append(newContent, '\n')
	}

	// Загрузить обновленный контент
	return l.uploadContent(ctx, objectName, newContent)
}

// GetSagaSize возвращает размер файла саги в байтах
func (l *MinIOSagaLogger) GetSagaSize(ctx context.Context, accountID, chatID string) (int64, error) {
	objectName := l.getObjectName(accountID, chatID)

	objInfo, err := l.minioClient.StatObject(ctx, l.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to stat saga object: %w", err)
	}

	return objInfo.Size, nil
}
