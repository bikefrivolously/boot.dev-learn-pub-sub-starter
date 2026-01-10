package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int
type AckType int

const (
	QueueTypeDurable SimpleQueueType = iota
	QueueTypeTransient
)

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonVal, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("error marshalling %v to JSON: %w", val, err)
	}

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        jsonVal,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing to channel: %s, %s, %v, %w", exchange, key, val, err)
	}
	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {

	channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error creating channel: %w", err)
	}

	var durable, autoDelete, exclusive, noWait bool

	switch queueType {
	case QueueTypeDurable:
		durable = true
		autoDelete = false
		exclusive = false
		noWait = false
	case QueueTypeTransient:
		durable = false
		autoDelete = true
		exclusive = true
		noWait = false
	}
	args := make(amqp.Table)
	args["x-dead-letter-exchange"] = "peril_dlx"
	q, err := channel.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, args)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error creating queue: %w", err)
	}

	err = channel.QueueBind(q.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error binding queue: %w", err)
	}
	return channel, q, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	channel, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return fmt.Errorf("error declaring or binding queue: %w", err)
	}
	deliveryChan, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("error while calling Consume: %w", err)
	}

	go func() {
		for d := range deliveryChan {
			var body T
			dErr := json.Unmarshal(d.Body, &body)
			if dErr != nil {
				fmt.Printf("error unmarshalling message into type %T: %v", body, dErr)
				continue
			}
			ackType := handler(body)
			switch ackType {
			case Ack:
				fmt.Println("Ack")
				d.Ack(false)
			case NackRequeue:
				fmt.Println("NackReque")
				d.Nack(false, true)
			default:
				fmt.Println("NackDiscard")
				d.Nack(false, false)
			}
		}
	}()
	return nil
}
