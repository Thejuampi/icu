package main

import icu "github.com/Thejuampi/icu"

func init() {
	RegisterCommand("chats", "list", &Command{
		Name:        "",
		Usage:       "chats list",
		Description: "List chats for icu.Athlete.",
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var c []icu.Chat
			if err := client.Get("chats", nil, nil, &c); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), c)
		},
	})

	RegisterCommand("chats", "get", &Command{
		Name:        "",
		Usage:       "chats get <id>",
		Description: "Get icu.Chat by ID.",
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Chat id")
			}

			var c icu.Chat
			if err := client.Get("chats", []string{args[0]}, nil, &c); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), c)
		},
	})

	RegisterCommand("chats", "messages", &Command{
		Name:        "",
		Usage:       "chats messages <id> [--limit N]",
		Description: "List messages in icu.Chat.",
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing("icu.Chat id")
			}

			q := queryFromFlags(flags, "limit")

			var msgs []icu.Message
			if err := client.Get("chats", []string{args[0], "messages"}, q, &msgs); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), msgs)
		},
	})

	RegisterCommand("chats", "send", &Command{
		Name:        "",
		Usage:       "chats send --content MSG [--to ID]",
		Description: "Send a icu.Message.",
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			var m icu.NewMessage
			m.Content = icu.StringFlag(flags, "content", "")
			m.ToAthleteID = icu.StringFlag(flags, "to", "")

			var resp icu.SendResponse
			if err := client.Post("chats", []string{"send-icu.Message"}, nil, m, &resp); err != nil {
				return err
			}

			return icu.WriteJSON(osStdout(), resp)
		},
	})
}
