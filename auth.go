package main

import (
	"encoding/base64"
	"fmt"
)

func BuildAuthHeader(apiKey string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("API_KEY:%s", apiKey)))
	return fmt.Sprintf("Basic %s", encoded)
}
