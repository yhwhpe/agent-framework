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
	// Extract chatID from event
	chatID, ok := event.Metadata["chatId"].(string)
	if !ok {
		return fmt.Errorf("chatId not found in event metadata")
	}

	// Extract participantId for metadata
	participantID, _ := event.Metadata["participantId"].(string)

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