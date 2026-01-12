package main

import (
	"fmt"
	"strconv"
	"time"

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
	warQueueName := routing.WarRecognitionsPrefix

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
		handlerMove(gameState, channel),
	)
	if err != nil {
		fmt.Printf("error subscribing to JSON moves queue: %v\n", err)
		return
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		warQueueName,
		routing.WarRecognitionsPrefix+".*",
		pubsub.QueueTypeDurable,
		handlerWar(gameState, channel),
	)
	if err != nil {
		fmt.Printf("error subscribing to JSON war queue: %v\n", err)
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
			if len(inputWords) != 2 {
				fmt.Printf("usage: spam <number>\n")
				continue
			}
			n, err := strconv.Atoi(inputWords[1])
			if err != nil {
				fmt.Printf("can't convert %s to number: %v\n", inputWords[1], err)
				continue
			}
			for ; n > 0; n-- {
				log := gamelogic.GetMaliciousLog()
				pubGameLog(channel, gameState.GetUsername(), log)
			}
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

func handlerMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	f := func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			msg := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			}
			exchange := routing.ExchangePerilTopic
			key := routing.WarRecognitionsPrefix + "." + gs.GetUsername()
			err := pubsub.PublishJSON(channel, exchange, key, msg)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
	return f
}

func handlerWar(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	f := func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		outcome, winner, loser := gs.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			// we're not involved, let another client pick this up
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeDraw:
			msg := fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			err := pubGameLog(channel, gs.GetUsername(), msg)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeOpponentWon:
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubGameLog(channel, gs.GetUsername(), msg)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)
			err := pubGameLog(channel, gs.GetUsername(), msg)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			fmt.Printf("error, unknown war outcome: %v\n", outcome)
			return pubsub.NackDiscard
		}
	}
	return f
}

func pubGameLog(ch *amqp.Channel, userName, msg string) error {
	exchange := routing.ExchangePerilTopic
	key := routing.GameLogSlug + "." + userName
	gl := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     msg,
		Username:    userName,
	}
	return pubsub.PublishGob(ch, exchange, key, gl)
}
