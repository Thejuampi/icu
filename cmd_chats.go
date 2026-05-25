package main

func init() {
	RegisterCommand("chats", "list", &Command{
		Name:        "",
		Usage:       "chats list",
		Description: "List chats for athlete.",
		Run: func(_ []string, _ map[string]string, client *Client) error {
			var c []Chat
			if err := client.Get("chats", nil, nil, &c); err != nil {
				return err
			}

			return WriteJSON(osStdout(), c)
		},
	})

	RegisterCommand("chats", "get", &Command{
		Name:        "",
		Usage:       "chats get <id>",
		Description: "Get chat by ID.",
		Run: func(args []string, _ map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("chat id")
			}

			var c Chat
			if err := client.Get("chats", []string{args[0]}, nil, &c); err != nil {
				return err
			}

			return WriteJSON(osStdout(), c)
		},
	})

	RegisterCommand("chats", "messages", &Command{
		Name:        "",
		Usage:       "chats messages <id> [--limit N]",
		Description: "List messages in chat.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			if len(args) == 0 {
				return errMissing("chat id")
			}

			q := queryFromFlags(flags, "limit")

			var msgs []Message
			if err := client.Get("chats", []string{args[0], "messages"}, q, &msgs); err != nil {
				return err
			}

			return WriteJSON(osStdout(), msgs)
		},
	})

	RegisterCommand("chats", "send", &Command{
		Name:        "",
		Usage:       "chats send --content MSG [--to ID]",
		Description: "Send a message.",
		Run: func(_ []string, flags map[string]string, client *Client) error {
			var m NewMessage
			m.Content = StringFlag(flags, "content", "")
			m.ToAthleteID = StringFlag(flags, "to", "")

			var resp SendResponse
			if err := client.Post("chats", []string{"send-message"}, nil, m, &resp); err != nil {
				return err
			}

			return WriteJSON(osStdout(), resp)
		},
	})
}
