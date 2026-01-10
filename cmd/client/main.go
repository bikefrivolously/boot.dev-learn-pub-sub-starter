package main

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/gamelogic"
	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/pubsub"
	"github.com/bikefrivolously/boot.dev-learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril client...")
	amqpConnection := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(amqpConnection)
	if err != nil {
		fmt.Printf("unable to connect to AMQP server %s, %v\n", amqpConnection, err)
		return
	}
	defer shutdown(conn)

	fmt.Printf("Connected to AMQP server: %s\n", amqpConnection)

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("error getting username: %v\n", err)
		return
	}

	queueName := routing.PauseKey + "." + userName
	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.QueueTypeTransient,
	)
	if err != nil {
		fmt.Printf("error creating or binding queue: %v\n", err)
		return
	}

	gameState := gamelogic.NewGameState(userName)

	running := true
	for running {
		inputWords := gamelogic.GetInput()
		switch inputWords[0] {
		case "spawn":
			err := gameState.CommandSpawn(inputWords)
			if err != nil {
				fmt.Printf("error in spawn command: %v\n", err)
			}
		case "move":
			move, err := gameState.CommandMove(inputWords)
			if err != nil {
				fmt.Printf("error in move command: %v\n", err)
				continue
			}
			fmt.Printf("move: %s %d\n", move.ToLocation, len(move.Units))
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			running = false
		default:
			fmt.Println("unrecognized command")
		}
	}
}

func shutdown(conn *amqp.Connection) {
	defer conn.Close()
	fmt.Println("Shutting down Peril client...")
}
