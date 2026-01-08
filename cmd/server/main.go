package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	amqpConnection := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(amqpConnection)
	if err != nil {
		fmt.Printf("unable to connect to AMQP server %s, %w\n", amqpConnection, err)
		return
	}
	defer conn.Close()

	fmt.Printf("Connected to AMQP server: %s\n", amqpConnection)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT)
	_ = <-shutdown
	fmt.Println("Shutting down Peril server...")
}
