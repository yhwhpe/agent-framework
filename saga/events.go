package saga

import (
	"time"
)

// BaseEvent представляет базовую структуру для всех событий саги
type BaseEvent struct {
	EventType string `json:"eventType"`
	Timestamp int64  `json:"timestamp"`
}

// PhaseStartedEvent - начало новой фазы онбординга
type PhaseStartedEvent struct {
	BaseEvent
	Phase string `json:"phase"` // "name" | "avatar" | "interview" | "email"
}

// PhaseCompletedEvent - завершение фазы с успешным сбором данных
type PhaseCompletedEvent struct {
	BaseEvent
	Phase string `json:"phase"` // "name" | "avatar" | "interview" | "email"
}

// LLMRequestEvent - запрос к LLM для генерации сообщения или анализа
type LLMRequestEvent struct {
	BaseEvent
	Phase       string            `json:"phase"`       // Текущая фаза
	RequestID   string            `json:"requestId"`   // UUID запроса
	Prompt      string            `json:"prompt"`      // Текст промпта
	Variables   map[string]string `json:"variables"`   // Переменные для подстановки
	Temperature float64           `json:"temperature"` // Температура генерации
	MaxTokens   int               `json:"maxTokens"`   // Максимальное кол-во токенов
	Tone        string            `json:"tone"`        // Тон сообщения
}

// LLMResponseEvent - ответ от LLM с сгенерированным контентом
type LLMResponseEvent struct {
	BaseEvent
	Phase        string  `json:"phase"`        // Текущая фаза
	RequestID    string  `json:"requestId"`    // UUID соответствующего запроса
	Content      string  `json:"content"`      // Сгенерированный текст
	TokensUsed   int     `json:"tokensUsed"`   // Количество использованных токенов
	ResponseTime float64 `json:"responseTime"` // Время генерации в секундах
	Model        string  `json:"model"`        // Использованная модель
}

// MessageAddedEvent - сообщения пользователей и агентов в чате (унаследован из Alura)
type MessageAddedEvent struct {
	BaseEvent
	Role    string `json:"role"`    // "user" | "assistant" | "system"
	Content string `json:"content"` // Текстовое содержимое сообщения
}

// CollectedDataEvent - событие сбора/обновления данных профиля
type CollectedDataEvent struct {
	BaseEvent
	Phase         string      `json:"phase"`                   // Фаза, в которой собраны данные
	Field         string      `json:"field"`                   // Название поля ("name", "avatar", etc.)
	Value         interface{} `json:"value"`                   // Значение поля
	Source        string      `json:"source"`                  // Источник данных ("user_input", "llm_inference", "external_api")
	Confidence    float64     `json:"confidence"`              // Уровень уверенности (0.0-1.0)
	PreviousValue interface{} `json:"previousValue,omitempty"` // Предыдущее значение (для обновлений)
}

// MesaAnalysisEvent - результаты анализа через Mesa API (для фазы интервью)
type MesaAnalysisEvent struct {
	BaseEvent
	Phase            string  `json:"phase"`            // Всегда "interview"
	SessionID        string  `json:"sessionId"`        // ID сессии Mesa
	Goal             string  `json:"goal"`             // Цель анализа
	Issue            string  `json:"issue"`            // Проблема
	MSCore           string  `json:"msCore"`           // MSCore метрика
	Psychotype       string  `json:"psychotype"`       // Психотип
	Confidence       float64 `json:"confidence"`       // Уровень уверенности
	ConsistencyScore float64 `json:"consistencyScore"` // Оценка консистентности
	Success          bool    `json:"success"`          // true если consistency >= 0.8
}

// AximaMilestoneEvent - достижение этапов в Axima после завершения интервью
type AximaMilestoneEvent struct {
	BaseEvent
	Phase                string `json:"phase"`                // Всегда "interview"
	MapID                string `json:"mapId"`                // ID карты milestone
	MilestoneTitle       string `json:"milestoneTitle"`       // Заголовок milestone
	MilestoneDescription string `json:"milestoneDescription"` // Описание milestone
}

// EventUnion - объединение всех типов событий для удобства работы
type EventUnion interface {
	GetEventType() string
	GetTimestamp() int64
}

// Реализация интерфейса EventUnion для всех событий
func (e PhaseStartedEvent) GetEventType() string { return e.EventType }
func (e PhaseStartedEvent) GetTimestamp() int64  { return e.Timestamp }

func (e PhaseCompletedEvent) GetEventType() string { return e.EventType }
func (e PhaseCompletedEvent) GetTimestamp() int64  { return e.Timestamp }

func (e LLMRequestEvent) GetEventType() string { return e.EventType }
func (e LLMRequestEvent) GetTimestamp() int64  { return e.Timestamp }

func (e LLMResponseEvent) GetEventType() string { return e.EventType }
func (e LLMResponseEvent) GetTimestamp() int64  { return e.Timestamp }

func (e MessageAddedEvent) GetEventType() string { return e.EventType }
func (e MessageAddedEvent) GetTimestamp() int64  { return e.Timestamp }

func (e CollectedDataEvent) GetEventType() string { return e.EventType }
func (e CollectedDataEvent) GetTimestamp() int64  { return e.Timestamp }

func (e MesaAnalysisEvent) GetEventType() string { return e.EventType }
func (e MesaAnalysisEvent) GetTimestamp() int64  { return e.Timestamp }

func (e AximaMilestoneEvent) GetEventType() string { return e.EventType }
func (e AximaMilestoneEvent) GetTimestamp() int64  { return e.Timestamp }

// NewTimestamp создает timestamp для текущего времени
func NewTimestamp() int64 {
	return time.Now().Unix()
}

// NewBaseEvent создает базовое событие с текущим timestamp
func NewBaseEvent(eventType string) BaseEvent {
	return BaseEvent{
		EventType: eventType,
		Timestamp: NewTimestamp(),
	}
}
