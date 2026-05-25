package main

func init() {
	RegisterCommand("chats", "list", &Command{
		Usage:       "chats list",
		Description: "List chats for athlete.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			var c []Chat
			if err := client.Get("chats", nil, nil, &c); err != nil {
				return err
			}
			return WriteJSON(osStdout(), c)
		},
	})

	RegisterCommand("chats", "get", &Command{
		Usage:       "chats get <id>",
		Description: "Get chat by ID.",
		Run: func(args []string, flags map[string]string, client *Client) error {
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
		Usage:       "chats send --content MSG [--to ID]",
		Description: "Send a message.",
		Run: func(args []string, flags map[string]string, client *Client) error {
			m := NewMessage{
				Content:     StringFlag(flags, "content", ""),
				ToAthleteID: StringFlag(flags, "to", ""),
			}
			var resp SendResponse
			if err := client.Post("chats", []string{"send-message"}, nil, m, &resp); err != nil {
				return err
			}
			return WriteJSON(osStdout(), resp)
		},
	})
}
