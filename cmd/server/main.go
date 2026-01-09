package main

import (
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/gamelogic"
	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/pubsub"
	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril server...")
	amqpConnection := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(amqpConnection)
	if err != nil {
		fmt.Printf("unable to connect to AMQP server %s, %v\n", amqpConnection, err)
		return
	}
	defer shutdown(conn)

	fmt.Printf("Connected to AMQP server: %s\n", amqpConnection)

	channel, err := conn.Channel()
	if err != nil {
		fmt.Printf("error creating channel: %v\n", err)
		return
	}

	gamelogic.PrintServerHelp()
	running := true
	for running {
		inputWords := gamelogic.GetInput()
		switch inputWords[0] {
		case "pause":
			pubPause(channel, true)
		case "resume":
			pubPause(channel, false)
		case "quit":
			running = false
		default:
			fmt.Printf("unrecognized command: %s\n", inputWords[0])
		}
	}
}

func shutdown(conn *amqp.Connection) {
	defer conn.Close()
	fmt.Println("Shutting down Peril server...")
}

func pubPause(c *amqp.Channel, paused bool) error {
	exchange := routing.ExchangePerilDirect
	key := routing.PauseKey
	val, err := json.Marshal(routing.PlayingState{IsPaused: paused})
	if err != nil {
		return fmt.Errorf("error marshalling value: %w\n", err)
	}

	err = pubsub.PublishJSON(c, exchange, key, val)
	if err != nil {
		return fmt.Errorf("error publishing json: %w\n", err)
	}
	return nil
}
