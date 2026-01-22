package events

// EventType mirrors cli-flow-service contract (CLI_FLOW_*).
type EventType string

const (
	CliFlowInit     EventType = "CLI_FLOW_INIT"
	CliFlowContinue EventType = "CLI_FLOW_CONTINUE"
	CliFlowEnd      EventType = "CLI_FLOW_END"
)

// Event matches cli-flow-service event envelope for compatibility across agents.
type Event struct {
	EventType     EventType      `json:"eventType"`
	EventID       string         `json:"eventId"`
	Timestamp     *int64         `json:"timestamp,omitempty"`
	FlowID        string         `json:"flowId"`
	ChatID        string         `json:"chatId"`
	ParticipantID string         `json:"participantId"`
	AccountID     string         `json:"accountId"`         // account ID from cli-flow-service
	Message       *string        `json:"message,omitempty"` // required for CONTINUE
	ActionType    string         `json:"actionType,omitempty"`
	ActionData    map[string]any `json:"actionData,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"` // communicator metadata from user message
}
