package communicator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
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
	log.Printf("📤 [COMMUNICATOR] Starting message send to chat %s: %d content items, source=%s, eventType=%s",
		input.ChatID, len(input.Contents), input.Source, input.EventType)

	// Validate input
	if input.ChatID == "" {
		log.Printf("❌ [COMMUNICATOR] Error: ChatID is empty")
		return fmt.Errorf("chat ID cannot be empty")
	}

	if len(input.Contents) == 0 {
		log.Printf("❌ [COMMUNICATOR] Error: No content items provided")
		return fmt.Errorf("at least one content item is required")
	}

	// Convert agent-framework content items to communicator format
	contents := make([]map[string]interface{}, len(input.Contents))
	for i, content := range input.Contents {
		log.Printf("📤 [COMMUNICATOR] Processing content %d: Type=%s, Order=%d", i+1, content.Type, content.Order)
		log.Printf("📤 [COMMUNICATOR] Content.Content field: '%s'", content.Content)
		log.Printf("📤 [COMMUNICATOR] Content.Data keys: %v", getMapKeys(content.Data))

		contents[i] = map[string]interface{}{
			"type":  content.Type,
			"order": content.Order,
			"data":  content.Data,
		}

		// Extract text content - check Content field first, then Data["text"]
		textContent := content.Content
		if textContent == "" && content.Data != nil {
			log.Printf("📤 [COMMUNICATOR] Content.Content is empty, checking Data for text...")
			if textVal, ok := content.Data["text"].(string); ok {
				textContent = textVal
				log.Printf("📤 [COMMUNICATOR] Found text in Data['text']: '%s'", textContent[:min(50, len(textContent))]+"...")
			} else {
				log.Printf("📤 [COMMUNICATOR] No 'text' key in Data, available keys: %v", getMapKeys(content.Data))
				// Try other possible keys
				for key, value := range content.Data {
					if strVal, ok := value.(string); ok && len(strVal) > 10 {
						log.Printf("📤 [COMMUNICATOR] Possible text content in Data['%s']: '%s'...", key, strVal[:min(50, len(strVal))])
					}
				}
			}
		} else {
			log.Printf("📤 [COMMUNICATOR] Using Content field: '%s'", textContent[:min(50, len(textContent))]+"...")
		}

		// Put text content into GraphQL data.content field
		if textContent != "" {
			if contents[i]["data"] == nil {
				contents[i]["data"] = make(map[string]interface{})
			}
			contents[i]["data"].(map[string]interface{})["content"] = textContent
		}

		// Warn if content is empty
		if textContent == "" {
			log.Printf("⚠️ [COMMUNICATOR] Warning: Content %d has no text content (Content field empty and no text in Data)", i+1)
		}

		// Log content details with content preview (after text extraction)
		contentPreview := textContent
		if len(textContent) > 100 {
			contentPreview = textContent[:100] + "..."
		}
		log.Printf("📤 [COMMUNICATOR] Content %d: type=%s, order=%d, content='%s'",
			i+1, content.Type, content.Order, contentPreview)
	}

	// Prepare GraphQL mutation
	query := `
		mutation AddAgentMessage($input: AgentMessageInput!) {
			addAgentMessage(input: $input) {
				success
				message {
					id
					chatId
				}
				error
			}
		}
	`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"chatId":   input.ChatID,
			"contents": contents,
			"metadata": input.Metadata,
		},
	}

	// Create HTTP request
	reqBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("❌ [COMMUNICATOR] Error marshaling GraphQL request: %v", err)
		return fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	url := c.baseURL + "/graphql"
	log.Printf("📤 [COMMUNICATOR] Making HTTP POST to %s", url)
	log.Printf("📤 [COMMUNICATOR] Request body size: %d bytes", len(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ [COMMUNICATOR] Error creating HTTP request: %v", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		log.Printf("📤 [COMMUNICATOR] Using API key for authorization")
	} else {
		log.Printf("⚠️ [COMMUNICATOR] Warning: No API key provided")
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [COMMUNICATOR] HTTP request failed: %v", err)
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("📥 [COMMUNICATOR] HTTP response status: %s", resp.Status)

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [COMMUNICATOR] HTTP error status: %d", resp.StatusCode)
		return fmt.Errorf("HTTP error status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ [COMMUNICATOR] Error reading response body: %v", err)
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("📥 [COMMUNICATOR] Response body size: %d bytes", len(body))
	log.Printf("📥 [COMMUNICATOR] Raw response: %s", string(body))

	// Parse response
	var response struct {
		Data struct {
			AddAgentMessage struct {
				Success bool `json:"success"`
				Message struct {
					ID     string `json:"id"`
					ChatID string `json:"chatId"`
				} `json:"message,omitempty"`
				Error string `json:"error,omitempty"`
			} `json:"addAgentMessage"`
		} `json:"data,omitempty"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("❌ [COMMUNICATOR] Error parsing JSON response: %v", err)
		log.Printf("❌ [COMMUNICATOR] Raw response that failed to parse: %s", string(body))
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		log.Printf("❌ [COMMUNICATOR] GraphQL validation/parsing errors: %v", response.Errors)
		for i, gqlErr := range response.Errors {
			log.Printf("❌ [COMMUNICATOR] GraphQL error %d: %s", i+1, gqlErr.Message)
		}
		return fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	// Check if addAgentMessage data exists
	if response.Data.AddAgentMessage.Success == false && response.Data.AddAgentMessage.Error == "" {
		log.Printf("❌ [COMMUNICATOR] No response data from addAgentMessage mutation")
		log.Printf("❌ [COMMUNICATOR] Full response structure: %+v", response)
		return fmt.Errorf("no response data from communicator")
	}

	// Check for application errors
	if !response.Data.AddAgentMessage.Success {
		errorMsg := response.Data.AddAgentMessage.Error
		if errorMsg == "" {
			errorMsg = "unknown application error"
		}
		log.Printf("❌ [COMMUNICATOR] Application error from communicator: %s", errorMsg)
		log.Printf("❌ [COMMUNICATOR] Full response data: %+v", response.Data.AddAgentMessage)
		return fmt.Errorf("communicator error: %s", errorMsg)
	}

	// Log success
	log.Printf("✅ [COMMUNICATOR] Message sent successfully!")
	log.Printf("✅ [COMMUNICATOR] Message ID: %s, Chat ID: %s",
		response.Data.AddAgentMessage.Message.ID,
		response.Data.AddAgentMessage.Message.ChatID)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getMapKeys(m map[string]interface{}) []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
