package feed

import (
	"context"
	"encoding/json"
	"fmt"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// FeedMessage represents the payload delivered via RabbitMQ to a subscriber.
type FeedMessage struct {
	Type   string            `json:"type"`
	UserID uuid.UUID         `json:"user_id"`
	Post   models.GetPostDTO `json:"post"`
}

// RabbitPublisher publishes notifications to a topic exchange with routing key = subscriber user ID.
type RabbitPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

// NewRabbitPublisher establishes a connection to RabbitMQ, declares an exchange, and returns a publisher.
func NewRabbitPublisher(url, exchange string) (*RabbitPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(
		exchange,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,   // args
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &RabbitPublisher{conn: conn, channel: ch, exchange: exchange}, nil
}

// PublishPostForUser publishes a notification for a specific subscriber.
func (p *RabbitPublisher) PublishPostForUser(subscriberID uuid.UUID, post models.GetPostDTO) error {
	message := FeedMessage{
		Type:   "post_created",
		UserID: subscriberID,
		Post:   post,
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal feed message: %w", err)
	}

	routingKey := subscriberID.String()
	return p.channel.PublishWithContext(
		context.Background(),
		p.exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         payload,
			DeliveryMode: amqp.Persistent,
		},
	)
}

// Close closes the RabbitMQ channel and connection.
func (p *RabbitPublisher) Close() error {
	if err := p.channel.Close(); err != nil {
		p.conn.Close()
		return err
	}
	return p.conn.Close()
}
