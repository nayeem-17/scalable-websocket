package main

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

func (m *Manager) connectRabbitMQ(uri string) error {
	var err error
	m.amqpConn, err = amqp.Dial(uri)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	m.amqpChannel, err = m.amqpConn.Channel()
	if err != nil {
		m.amqpConn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	err = m.amqpChannel.ExchangeDeclare(
		broadcastExchange, // name
		"fanout",          // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		m.amqpChannel.Close()
		m.amqpConn.Close()
		return fmt.Errorf("failed to declare broadcast exchange: %w", err)
	}
	log.Println("RabbitMQ connected and broadcast exchange declared.")
	return nil

}
func (m *Manager) publishToRabbitMQ(exchange, routingKey string, body []byte) error {
	if m.amqpChannel == nil {
		return fmt.Errorf("RabbitMQ channel is not initialized")
	}
	err := m.amqpChannel.Publish(
		exchange,   // exchange
		routingKey, // routing key (ignored by fanout, used by topic/direct)
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return fmt.Errorf("failed to publish message to RabbitMQ: %w", err)
	}
	// log.Printf("Published message to RabbitMQ exchange '%s': %s\n", exchange, string(body))
	return nil
}

func (m *Manager) consumeFromRabbitMQ() {
	if m.amqpChannel == nil {
		log.Println("Cannot consume from RabbitMQ: channel not initialized.")
		return
	}

	// Declare a unique, auto-delete, exclusive queue for this server instance
	// to receive messages from the fanout exchange.
	q, err := m.amqpChannel.QueueDeclare(
		"",    // name (empty means RabbitMQ will generate a unique name)
		false, // durable
		true,  // delete when unused (auto-delete)
		true,  // exclusive (only this connection can use it)
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		log.Printf("Failed to declare a queue: %v\n", err)
		return
	}

	// Bind the queue to the fanout exchange
	err = m.amqpChannel.QueueBind(
		q.Name,            // queue name
		"",                // routing key (ignored for fanout)
		broadcastExchange, // exchange
		false,
		nil,
	)
	if err != nil {
		log.Printf("Failed to bind queue '%s' to exchange '%s': %v\n", q.Name, broadcastExchange, err)
		return
	}
	log.Printf("Declared queue '%s' and bound to exchange '%s'\n", q.Name, broadcastExchange)

	msgs, err := m.amqpChannel.Consume(
		q.Name, // queue
		"",     // consumer (empty for a server-generated consumer tag)
		true,   // auto-ack (set to false for manual acknowledgment if needed)
		false,  // exclusive
		false,  // no-local (RabbitMQ server will not send messages published by this connection back to it - useful)
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v\n", err)
		return
	}

	log.Println("RabbitMQ consumer started. Waiting for messages...")
	for {
		select {
		case <-m.shutdownChan: // Listen for manager shutdown
			log.Println("RabbitMQ consumer shutting down.")
			// Potential cleanup for consumer if necessary, though channel/connection close handles most
			return
		case d, ok := <-msgs:
			if !ok {
				log.Println("RabbitMQ consumer channel closed by server.")
				// Attempt to reconnect or handle error
				return
			}
			// log.Printf("Received message from RabbitMQ: %s\n", string(d.Body))
			// Send this message to all locally connected WebSocket clients
			// Ensure this doesn't block the consumer for too long.
			// The broadcast channel is buffered.
			select {
			case m.broadcast <- d.Body:
			default:
				log.Printf("Local broadcast channel full. Dropping message from RabbitMQ: %s\n", string(d.Body))
			}
		}
	}
}
