package icu

import "encoding/base64"

func BuildAuthHeader(apiKey string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte("API_KEY:" + apiKey))

	return "Basic " + encoded
}
