package main

import (
	"errors"
	"fmt"

	icu "github.com/Thejuampi/icu"
)

var errMissingRequired = errors.New("missing required argument")

func errMissing(what string) error {
	return fmt.Errorf("%w: %s", errMissingRequired, what)
}

func listAllCommand[T any](resource, usage, description string, parts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			var items T
			if err := client.Get(resource, parts, nil, &items); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(items)
		},
	}
}

func listQueryCommand[T any](resource, usage, description string, queryBuilder func(map[string]string) map[string]string, parts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := map[string]string(nil)
			if queryBuilder != nil {
				q = queryBuilder(flags)
			}

			var items T
			if err := client.Get(resource, parts, q, &items); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(items)
		},
	}
}

func getByIDCommand[T any](resource, usage, description, idLabel string, queryBuilder func(map[string]string) map[string]string, extraParts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing(idLabel)
			}

			parts := append([]string{args[0]}, extraParts...)

			q := map[string]string(nil)
			if queryBuilder != nil {
				q = queryBuilder(flags)
			}

			var item T
			if err := client.Get(resource, parts, q, &item); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(item)
		},
	}
}

func deleteByIDCommand(resource, usage, description, idLabel string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing(idLabel)
			}

			return wrapCommandError(client.Delete(resource, []string{args[0]}, nil, nil))
		},
	}
}

func deleteByIDWithResponseCommand[T any](resource, usage, description, idLabel string, extraParts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing(idLabel)
			}

			parts := append([]string{args[0]}, extraParts...)

			var resp T
			if err := client.Delete(resource, parts, nil, &resp); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(resp)
		},
	}
}

func downloadCommand(resource, usage, description string, parts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, _ map[string]string, client *icu.Client) error {
			data, err := client.Download(resource, parts, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	}
}

func downloadByIDCommand(resource, usage, description, idLabel string, extraParts ...string) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, _ map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing(idLabel)
			}

			parts := append([]string{args[0]}, extraParts...)

			data, err := client.Download(resource, parts, nil)
			if err != nil {
				return wrapCommandError(err)
			}

			return writeOutput(data)
		},
	}
}

func createCommand[Body, Resp any](
	resource, usage, description string,
	queryBuilder func(map[string]string) map[string]string,
	build func(map[string]string) Body,
	parts ...string,
) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(_ []string, flags map[string]string, client *icu.Client) error {
			q := map[string]string(nil)
			if queryBuilder != nil {
				q = queryBuilder(flags)
			}

			var result Resp
			if err := client.Post(resource, parts, q, build(flags), &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func updateByIDCommand[Body, Resp any](
	resource, usage, description, idLabel string,
	queryBuilder func(map[string]string) map[string]string,
	build func(map[string]string) Body,
	extraParts ...string,
) *Command {
	return &Command{
		Name:        "",
		Usage:       usage,
		Description: description,
		Run: func(args []string, flags map[string]string, client *icu.Client) error {
			if len(args) == 0 {
				return errMissing(idLabel)
			}

			parts := append([]string{args[0]}, extraParts...)

			q := map[string]string(nil)
			if queryBuilder != nil {
				q = queryBuilder(flags)
			}

			var result Resp
			if err := client.Put(resource, parts, q, build(flags), &result); err != nil {
				return wrapCommandError(err)
			}

			return writeJSON(result)
		},
	}
}

func queryBuilder(keys ...string) func(map[string]string) map[string]string {
	return func(flags map[string]string) map[string]string {
		return queryFromFlags(flags, keys...)
	}
}

func staticQuery(q map[string]string) func(map[string]string) map[string]string {
	return func(map[string]string) map[string]string {
		return q
	}
}
