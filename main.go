package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nlopes/slack"

	"github.com/domudall/doiici/plugins"

	// Initialise plugins
	_ "github.com/domudall/doiici/plugins/echo"
	_ "github.com/domudall/doiici/plugins/ping"
)

var (
	listeners = make(map[string]plugins.Plugin)

	dmChar = []byte("D")[0]
)

func main() {
	log.Println("loading plugins")
	for _, p := range plugins.Registry {
		name := strings.ToLower(p.GetName())
		log.Println(fmt.Sprintf("loading %q", name))

		if _, ok := listeners[name]; ok {
			log.Fatalf("plugin %q registered twice", name)
		}
		listeners[name] = p
	}

	if len(listeners) == 0 {
		log.Fatal("at least one plugin must be installed")
	}

	log.Println("plugins loaded!")

	log.Println("connecting to slack")
	api := slack.New(os.Getenv("BOT_TOKEN"))

	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)
	slack.SetLogger(logger)

	rtm := api.NewRTM()
	go rtm.ManageConnection()

	var info *slack.Info

	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.ConnectedEvent:
			info = ev.Info

		case *slack.MessageEvent:
			if info == nil {
				break
			}

			// If the message is from the bot itself, ignore
			if ev.User == info.User.ID {
				break
			}

			// If the first part of the message is @<botname>, it's a message at the bot
			atBot := strings.HasPrefix(ev.Text, fmt.Sprintf("<@%s>", info.User.ID))

			// If channel starts with "D", it's a direct message to the bot
			toBot := ev.Channel[0] == dmChar

			if !atBot && !toBot {
				break
			}

			partSplitCount := 2
			if atBot {
				partSplitCount = partSplitCount + 1
			}

			msgParts := strings.SplitN(ev.Text, " ", partSplitCount)
			if len(msgParts) < partSplitCount-1 {
				rtm.SendMessage(rtm.NewOutgoingMessage("Yes?", ev.Channel))
				break
			}

			plugin := strings.ToLower(msgParts[partSplitCount-2])
			p, ok := listeners[plugin]
			if !ok {
				msg := fmt.Sprintf("Sorry, the %q plugin doesn't seem to be installed.", plugin)
				rtm.SendMessage(rtm.NewOutgoingMessage(msg, ev.Channel))
				break
			}

			command := ""
			if len(msgParts) == partSplitCount {
				command = msgParts[partSplitCount-1]
			}

			params := slack.NewPostMessageParameters()
			params.AsUser = true

			msg := p.Match(command, params)
			params.Attachments = msg.Attachments

			api.PostMessage(ev.Channel, msg.Text, params)

		case *slack.RTMError:
			log.Println(fmt.Sprintf("Error: %s\n", ev.Error()))

		case *slack.InvalidAuthEvent:
			log.Println("Invalid credentials")
			return
		}
	}
}
