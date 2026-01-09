package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"

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

	exchange := routing.ExchangePerilDirect
	key := routing.PauseKey
	val, err := json.Marshal(routing.PlayingState{IsPaused: true})
	if err != nil {
		fmt.Printf("error marshalling value: %v\n", err)
		return
	}

	err = pubsub.PublishJSON(channel, exchange, key, val)
	if err != nil {
		fmt.Printf("error publishing json: %v\n", err)
		return
	}

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGTERM, syscall.SIGINT)
	<-shutdownSig
}

func shutdown(conn *amqp.Connection) {
	defer conn.Close()
	fmt.Println("Shutting down Peril server...")
}
