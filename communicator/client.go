package communicator

import (
	"context"
	"log"
)

// MessageContentItemInput represents input for message content
type MessageContentItemInput struct {
	Type    string                 `json:"type"`
	Content string                 `json:"content,omitempty"`
	Order   int                    `json:"order,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// AgentMessageInput represents input for agent message
type AgentMessageInput struct {
	ChatID    string                    `json:"chatId"`
	Contents  []MessageContentItemInput `json:"contents"`
	Metadata  map[string]interface{}    `json:"metadata,omitempty"`
	Source    string                    `json:"source"`
	EventType string                    `json:"eventType"`
}

// Client represents the communicator client
type Client struct {
	baseURL string
	apiKey  string
}

// New creates a new communicator client
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// AddAgentMessage sends a message to the communicator
func (c *Client) AddAgentMessage(ctx context.Context, input AgentMessageInput) error {
	log.Printf("📤 [COMMUNICATOR] Sending message to chat %s: %d content items", input.ChatID, len(input.Contents))

	// TODO: Implement actual HTTP call to communicator API
	// For now, just log the message
	for i, content := range input.Contents {
		log.Printf("📤 [COMMUNICATOR] Content %d: type=%s, content=%s", i+1, content.Type, content.Content[:min(100, len(content.Content))])
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
