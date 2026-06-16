package main

import icu "github.com/Thejuampi/icu"

func registerChatsCommands(registry *CommandRegistry) {
	registry.Register("chats", "list", listAllCommand[[]icu.Chat]("chats", "chats list", "List chats."))
	registry.Register("chats", "get", getByIDCommand[icu.Chat]("chats", "chats get <id>", "Get chat by ID.", "icu.Chat id", nil))

	registry.Register("chats", "messages", &Command{
		Name:        "",
		Usage:       "chats messages <id> [--limit N]",
		Description: "List messages in a chat.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Chat id")
			}

			q := queryFromFlags(flags, "limit")

			var msgs []icu.Message
			if err := client.Get("chats", []string{args[0], "messages"}, q, &msgs); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(msgs)
		},
	})

	registry.Register("chats", "send", createCommand[icu.NewMessage, icu.SendResponse](
		"chats",
		"chats send --content MSG [--to ID]",
		"Send a message.",
		staticQuery(map[string]string{}),
		func(flags map[string]string) icu.NewMessage {
			return icu.NewMessage{
				Content:     icu.StringFlag(flags, "content", ""),
				ToAthleteID: icu.StringFlag(flags, "to", ""),
			}
		},
		"send-message",
	))
}
