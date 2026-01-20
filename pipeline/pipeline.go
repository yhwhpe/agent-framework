package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/yhwhpe/agent-framework/communicator"
	framework_events "github.com/yhwhpe/agent-framework/events"
)

// ProcessingStage defines the interface for processing stage
type ProcessingStage interface {
	ProcessInit(ctx context.Context, event framework_events.Event) (*ProcessingResult, error)
	ProcessContinue(ctx context.Context, event framework_events.Event) (*ProcessingResult, error)
}

// PostProcessingStage defines the interface for postprocessing stage
type PostProcessingStage interface {
	PostProcessInit(ctx context.Context, event framework_events.Event, result *ProcessingResult) error
	PostProcessContinue(ctx context.Context, event framework_events.Event, result *ProcessingResult) error
}

// ProcessingResult contains the result of processing
type ProcessingResult struct {
	Contents        []communicator.MessageContentItemInput `json:"contents,omitempty"`
	Data            map[string]interface{}                 `json:"data,omitempty"`
	Metadata        map[string]interface{}                 `json:"metadata,omitempty"`
	PostProcessData map[string]interface{}                 `json:"postProcessData,omitempty"`
}

// PipelineStep defines a single step in the pipeline
type PipelineStep struct {
	Name           string
	Processing     ProcessingFunc
	PostProcessing PostProcessingFunc
}

// ProcessingFunc defines processing function type
type ProcessingFunc func(ctx context.Context, event framework_events.Event) (*ProcessingResult, error)

// PostProcessingFunc defines postprocessing function type
type PostProcessingFunc func(ctx context.Context, event framework_events.Event, result *ProcessingResult) error

// Pipeline manages the execution of processing and postprocessing stages
type Pipeline struct {
	communicator *communicator.Client
	step         PipelineStep
}

// New creates a new pipeline
func New(comm *communicator.Client, step PipelineStep) *Pipeline {
	return &Pipeline{
		communicator: comm,
		step:         step,
	}
}

// ExecuteInit executes the init pipeline for an event
func (p *Pipeline) ExecuteInit(ctx context.Context, event framework_events.Event) error {
	log.Printf("🔄 [PIPELINE] Executing INIT pipeline for event %s", event.EventID)

	// Processing phase
	result, err := p.step.Processing(ctx, event)
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	// Log processing result
	if result != nil {
		log.Printf("📦 [PIPELINE] Processing result: %d contents, metadata keys=%v",
			len(result.Contents), getMapKeys(result.Metadata))

		// Log each content item
		for i, content := range result.Contents {
			log.Printf("📦 [PIPELINE] Content %d: Type=%s, Order=%d, Data keys=%v",
				i+1, content.Type, content.Order, getMapKeys(content.Data))

			// Log text content if available
			if text, ok := content.Data["text"].(string); ok {
				log.Printf("📦 [PIPELINE] Content %d text (first 100): %s...",
					i+1, text[:min(100, len(text))])
			}
		}
	} else {
		log.Printf("📦 [PIPELINE] Processing result is nil")
	}

	// Send message to communicator if contents are present
	if result != nil && len(result.Contents) > 0 {
		log.Printf("📤 [PIPELINE] Sending %d contents to communicator for chat %s",
			len(result.Contents), event.ChatID)
		err = p.sendToCommunicator(ctx, event, result.Contents)
		if err != nil {
			log.Printf("⚠️ [PIPELINE] Failed to send message to communicator: %v", err)
		}
	} else {
		log.Printf("📦 [PIPELINE] No contents to send to communicator")
	}

	// Postprocessing phase
	if p.step.PostProcessing != nil {
		err = p.step.PostProcessing(ctx, event, result)
		if err != nil {
			return fmt.Errorf("postprocessing failed: %w", err)
		}
	}

	log.Printf("✅ [PIPELINE] INIT pipeline completed for event %s", event.EventID)
	return nil
}

// ExecuteContinue executes the continue pipeline for an event
func (p *Pipeline) ExecuteContinue(ctx context.Context, event framework_events.Event) error {
	log.Printf("🔄 [PIPELINE] Executing CONTINUE pipeline for event %s", event.EventID)

	// Processing phase
	result, err := p.step.Processing(ctx, event)
	if err != nil {
		return fmt.Errorf("processing failed: %w", err)
	}

	// Send message to communicator if contents are present
	if result != nil && len(result.Contents) > 0 {
		err = p.sendToCommunicator(ctx, event, result.Contents)
		if err != nil {
			log.Printf("⚠️ [PIPELINE] Failed to send message to communicator: %v", err)
		}
	}

	// Postprocessing phase
	if p.step.PostProcessing != nil {
		err = p.step.PostProcessing(ctx, event, result)
		if err != nil {
			return fmt.Errorf("postprocessing failed: %w", err)
		}
	}

	log.Printf("✅ [PIPELINE] CONTINUE pipeline completed for event %s", event.EventID)
	return nil
}

// sendToCommunicator sends contents to the communicator
func (p *Pipeline) sendToCommunicator(ctx context.Context, event framework_events.Event, contents []communicator.MessageContentItemInput) error {
	// Extract chatID from event (primary field, not metadata)
	chatID := event.ChatID
	if chatID == "" {
		return fmt.Errorf("chatId not found in event")
	}

	// Extract participantId for metadata
	participantID := event.ParticipantID

	messageInput := communicator.AgentMessageInput{
		ChatID:   chatID,
		Contents: contents,
		Metadata: map[string]interface{}{
			"participantId": participantID,
			"eventType":     event.EventType,
			"source":        "profile-builder",
		},
		Source:    "profile-builder",
		EventType: string(event.EventType),
	}

	return p.communicator.AddAgentMessage(ctx, messageInput)
}

// CreateProcessingFunc creates a processing function from a ProcessingStage
func CreateProcessingFunc(stage ProcessingStage) ProcessingFunc {
	return func(ctx context.Context, event framework_events.Event) (*ProcessingResult, error) {
		switch event.EventType {
		case "CLI_FLOW_INIT":
			return stage.ProcessInit(ctx, event)
		case "CLI_FLOW_CONTINUE":
			return stage.ProcessContinue(ctx, event)
		default:
			return nil, fmt.Errorf("unsupported event type: %s", event.EventType)
		}
	}
}

// CreatePostProcessingFunc creates a postprocessing function from a PostProcessingStage
func CreatePostProcessingFunc(stage PostProcessingStage) PostProcessingFunc {
	return func(ctx context.Context, event framework_events.Event, result *ProcessingResult) error {
		switch event.EventType {
		case "CLI_FLOW_INIT":
			return stage.PostProcessInit(ctx, event, result)
		case "CLI_FLOW_CONTINUE":
			return stage.PostProcessContinue(ctx, event, result)
		default:
			return fmt.Errorf("unsupported event type: %s", event.EventType)
		}
	}
}

// Helper functions
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
