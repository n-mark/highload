package ws

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitConsumer consumes messages from RabbitMQ for bound user IDs and forwards them to the hub.
type RabbitConsumer struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	queueName string
	exchange  string
	hub       *Hub
}

// NewRabbitConsumer creates an exclusive auto-delete queue and starts consuming messages.
func NewRabbitConsumer(url, exchange string, hub *Hub) (*RabbitConsumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbit consumer: failed to connect: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbit consumer: failed to open channel: %w", err)
	}

	// Declare exclusive, autodelete, non-durable queue with generated name
	q, err := ch.QueueDeclare(
		"", // let server generate name
		false,
		true,
		true,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbit consumer: failed to declare queue: %w", err)
	}

	rc := &RabbitConsumer{conn: conn, channel: ch, queueName: q.Name, exchange: exchange, hub: hub}

	msgs, err := ch.Consume(
		rc.queueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		rc.Close()
		return nil, fmt.Errorf("rabbit consumer: failed to start consuming: %w", err)
	}

	// Start message loop
	go rc.loop(msgs)

	// set bind/unbind handlers
	hub.SetBindHandler(func(uid uuid.UUID) { rc.bind(uid) })
	hub.SetUnbindHandler(func(uid uuid.UUID) { rc.unbind(uid) })

	return rc, nil
}

func (rc *RabbitConsumer) bind(userID uuid.UUID) {
	routingKey := userID.String()
	if err := rc.channel.QueueBind(
		rc.queueName,
		routingKey,
		rc.exchange,
		false,
		nil,
	); err != nil {
		log.Printf("rabbit consumer: failed to bind key=%s: %v", routingKey, err)
	}
}

func (rc *RabbitConsumer) unbind(userID uuid.UUID) {
	routingKey := userID.String()
	if err := rc.channel.QueueUnbind(
		rc.queueName,
		routingKey,
		rc.exchange,
		nil,
	); err != nil {
		log.Printf("rabbit consumer: failed to unbind key=%s: %v", routingKey, err)
	}
}

func (rc *RabbitConsumer) loop(deliveries <-chan amqp.Delivery) {
	for d := range deliveries {
		var msg struct {
			Type   string          `json:"type"`
			UserID uuid.UUID       `json:"user_id"`
			Post   json.RawMessage `json:"post"`
		}
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			log.Printf("rabbit consumer: invalid message format: %v", err)
			continue
		}

		// forward raw JSON to client
		rc.hub.BroadcastToUser(msg.UserID, d.Body)
	}
}

// Close cleans up RabbitMQ resources.
func (rc *RabbitConsumer) Close() error {
	if err := rc.channel.Close(); err != nil {
		rc.conn.Close()
		return err
	}
	return rc.conn.Close()
}
