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

	channel, err := conn.Channel()
	if err != nil {
		fmt.Printf("error creating channel: %v\n", err)
		return
	}

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("error getting username: %v\n", err)
		return
	}

	gameState := gamelogic.NewGameState(userName)
	pauseQueueName := routing.PauseKey + "." + userName
	movesQueueName := routing.ArmyMovesPrefix + "." + userName

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		pauseQueueName,
		routing.PauseKey,
		pubsub.QueueTypeTransient,
		handlerPause(gameState),
	)
	if err != nil {
		fmt.Printf("error subscribing to JSON pause queue: %v\n", err)
		return
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		movesQueueName,
		routing.ArmyMovesPrefix+".*",
		pubsub.QueueTypeTransient,
		handlerMove(gameState),
	)
	if err != nil {
		fmt.Printf("error subscribing to JSON pause queue: %v\n", err)
		return
	}

	running := true
	for running {
		inputWords := gamelogic.GetInput()
		if len(inputWords) == 0 {
			continue
		}
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
			err = pubsub.PublishJSON(channel, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+userName, move)
			if err != nil {
				fmt.Printf("error publishing move: %v\n", err)
				continue
			}
			fmt.Printf("Successfully published move: %s %s\n", move.Player.Username, move.ToLocation)
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	f := func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
	return f
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	f := func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		if outcome == gamelogic.MoveOutComeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
			return pubsub.Ack
		}
		return pubsub.NackDiscard
	}
	return f
}
