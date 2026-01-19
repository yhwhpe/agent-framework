package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/yhwhpe/agent-framework/events"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Handler func(context.Context, events.Event) error

type Consumer struct {
	url      string
	exchange string
	queue    string
	bindings []string

	prefetch   int
	maxRetries int

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(url, exchange, queue string, bindings []string, prefetch, maxRetries int) *Consumer {
	if prefetch <= 0 {
		prefetch = 1
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Consumer{
		url:        url,
		exchange:   exchange,
		queue:      queue,
		bindings:   bindings,
		prefetch:   prefetch,
		maxRetries: maxRetries,
	}
}

func (c *Consumer) Ping(ctx context.Context) error {
	_ = ctx
	return c.ensureConnection()
}

func (c *Consumer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Consumer) ensureConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() && c.channel != nil && !c.channel.IsClosed() {
		return nil
	}

	conn, err := amqp.DialConfig(c.url, amqp.Config{Properties: amqp.Table{
		"connection_name": "profile-builder",
	}})
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("amqp channel: %w", err)
	}

	if err := ch.ExchangeDeclare(c.exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("exchange declare: %w", err)
	}

	q, err := ch.QueueDeclare(
		c.queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("queue declare: %w", err)
	}

	for _, rk := range c.bindings {
		if rk == "" {
			continue
		}
		if err := ch.QueueBind(q.Name, rk, c.exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return fmt.Errorf("queue bind %s: %w", rk, err)
		}
	}

	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("qos: %w", err)
	}

	c.conn = conn
	c.channel = ch
	return nil
}

func retryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	if v, ok := headers["x-retry-count"]; ok {
		switch t := v.(type) {
		case int32:
			return int(t)
		case int64:
			return int(t)
		case int:
			return t
		}
	}
	return 0
}

func setRetryCount(headers amqp.Table, n int) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	headers["x-retry-count"] = int32(n)
	return headers
}

func (c *Consumer) republish(ctx context.Context, d amqp.Delivery, retry int) error {
	c.mu.Lock()
	ch := c.channel
	c.mu.Unlock()
	if ch == nil || ch.IsClosed() {
		return errors.New("channel is closed")
	}

	h := setRetryCount(d.Headers, retry)
	return ch.PublishWithContext(
		ctx,
		d.Exchange,
		d.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  d.ContentType,
			Body:         d.Body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Headers:      h,
		},
	)
}

func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	log.Printf("[RABBITMQ] 🚀 Starting RabbitMQ consumer: exchange=%s, queue=%s, bindings=%v, prefetch=%d, maxRetries=%d",
		c.exchange, c.queue, c.bindings, c.prefetch, c.maxRetries)

	if c.url == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	if c.exchange == "" {
		return errors.New("RABBITMQ_EXCHANGE is required")
	}
	if c.queue == "" {
		return errors.New("RABBITMQ_QUEUE is required")
	}
	if len(c.bindings) == 0 {
		return errors.New("RABBITMQ_BINDINGS is required")
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := c.ensureConnection(); err != nil {
			log.Printf("[RABBITMQ] ❌ Connection failed, retrying in %v: %v", backoff, err)
			j := time.Duration(rand.Int63n(int64(backoff / 2)))
			time.Sleep(backoff + j)
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}
		log.Printf("[RABBITMQ] 🔗 Connection established successfully")
		backoff = time.Second

		c.mu.Lock()
		ch := c.channel
		queue := c.queue
		c.mu.Unlock()

		msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			log.Printf("[RABBITMQ] ❌ Consumer registration failed: %v", err)
			_ = ch.Close()
			continue
		}

		log.Printf("[RABBITMQ] 🎧 Consumer registered successfully, waiting for messages: queue=%s, exchange=%s, bindings=%v",
			queue, c.exchange, c.bindings)

		for {
			select {
			case <-ctx.Done():
				return nil
			case d, ok := <-msgs:
				if !ok {
					log.Printf("[RABBITMQ] 🔌 Delivery channel closed, reconnecting...")
					_ = ch.Close()
					goto reconnect
				}

				// Логируем входящее RabbitMQ сообщение
				log.Printf("[RABBITMQ] 📨 Received message: exchange=%s, routing_key=%s, content_type=%s, message_id=%s, timestamp=%v, delivery_tag=%d",
					d.Exchange, d.RoutingKey, d.ContentType, d.MessageId, d.Timestamp, d.DeliveryTag)

				// Логируем headers если они есть
				if len(d.Headers) > 0 {
					headersJSON, _ := json.Marshal(d.Headers)
					log.Printf("[RABBITMQ] 📋 Headers: %s", string(headersJSON))
				}

				// Логируем raw payload для отладки
				log.Printf("[RABBITMQ] 📦 Raw payload (%d bytes): %s", len(d.Body), string(d.Body))

				var ev events.Event
				if err := json.Unmarshal(d.Body, &ev); err != nil {
					log.Printf("[rabbitmq] ❌ invalid json: %v", err)
					_ = d.Nack(false, false)
					continue
				}

				// Детальное логирование распарсенного события
				log.Printf("[RABBITMQ] 🎯 Parsed event: eventId=%s, eventType=%s, flowId=%s, chatId=%s, participantId=%s, accountId=%s",
					ev.EventID, ev.EventType, ev.FlowID, ev.ChatID, ev.ParticipantID, ev.AccountID)

				if ev.Timestamp != nil {
					log.Printf("[RABBITMQ] ⏰ Event timestamp: %d", *ev.Timestamp)
				}

				if ev.Message != nil {
					log.Printf("[RABBITMQ] 💬 Message content: %s", *ev.Message)
				}

				if ev.Data != nil {
					dataJSON, _ := json.Marshal(ev.Data)
					log.Printf("[RABBITMQ] 📊 Event data: %s", string(dataJSON))
				}

				if ev.Metadata != nil {
					metadataJSON, _ := json.Marshal(ev.Metadata)
					log.Printf("[RABBITMQ] 🏷️  Event metadata: %s", string(metadataJSON))
				}

				if ev.EventID == "" || ev.FlowID == "" || ev.ChatID == "" || ev.ParticipantID == "" || ev.EventType == "" {
					log.Printf("[RABBITMQ] ❌ Invalid event payload (missing required fields): eventId=%q flowId=%q chatId=%q participantId=%q eventType=%q",
						ev.EventID, ev.FlowID, ev.ChatID, ev.ParticipantID, ev.EventType)
					log.Printf("[RABBITMQ] 🚫 Message NACKed (invalid payload) - delivery_tag=%d", d.DeliveryTag)
					_ = d.Nack(false, false)
					continue
				}

				// For CLI_FLOW_CONTINUE, Message is required UNLESS it's an action (has Data with actionType)
				if ev.EventType == events.CliFlowContinue {
					isAction := ev.Data != nil && ev.Data["actionType"] != nil
					if isAction {
						log.Printf("[RABBITMQ] 🎬 CLI_FLOW_CONTINUE action detected: eventId=%s, actionType=%v", ev.EventID, ev.Data["actionType"])
					}
					if !isAction && (ev.Message == nil || *ev.Message == "") {
						log.Printf("[RABBITMQ] ❌ CLI_FLOW_CONTINUE missing message (and not an action): eventId=%s", ev.EventID)
						log.Printf("[RABBITMQ] 🚫 Message NACKed (missing message) - delivery_tag=%d", d.DeliveryTag)
						_ = d.Nack(false, false)
						continue
					}
					// If it's an action, Message can be empty - data is in Data field
				}

				// Вызываем handler для обработки события
				handlerStartTime := time.Now()
				err := handler(ctx, ev)
				handlerDuration := time.Since(handlerStartTime)

				log.Printf("[RABBITMQ] ⚙️  Handler processing completed in %v: eventId=%s, success=%v",
					handlerDuration, ev.EventID, err == nil)

				if err := handler(ctx, ev); err != nil {
					rc := retryCount(d.Headers)
					log.Printf("[RABBITMQ] ❌ Handler failed: eventId=%s, retry_count=%d, error=%v", ev.EventID, rc, err)

					if rc >= c.maxRetries {
						log.Printf("[RABBITMQ] 💀 Max retries exceeded (%d), dropping message: eventId=%s", rc, ev.EventID)
						log.Printf("[RABBITMQ] ✅ Message ACKed (dropped) - delivery_tag=%d", d.DeliveryTag)
						_ = d.Ack(false)
						continue
					}

					log.Printf("[RABBITMQ] 🔄 Republishing message for retry: eventId=%s, new_retry_count=%d", ev.EventID, rc+1)
					repCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					repErr := c.republish(repCtx, d, rc+1)
					cancel()
					if repErr != nil {
						log.Printf("[RABBITMQ] ❌ Republish failed: eventId=%s, error=%v (nack requeue)", ev.EventID, repErr)
						log.Printf("[RABBITMQ] 🔄 Message NACKed (requeue) - delivery_tag=%d", d.DeliveryTag)
						_ = d.Nack(false, true)
						continue
					}
					log.Printf("[RABBITMQ] ✅ Message ACKed (republished) - delivery_tag=%d", d.DeliveryTag)
					_ = d.Ack(false)
					continue
				}

				log.Printf("[RABBITMQ] ✅ Handler successful, message ACKed: eventId=%s, delivery_tag=%d", ev.EventID, d.DeliveryTag)
				_ = d.Ack(false)
			}
		}

	reconnect:
		continue
	}
}
