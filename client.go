package icu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errHTTPStatus = errors.New("HTTP status error")

const defaultClientTimeout = 30 * time.Second

type Client struct {
	httpClient *http.Client
	apiKey     string
	athleteID  string
	baseURL    string
}

type ClientOption func(*Client)

// WithHTTPClient uses httpClient for API calls. A client without a
// CheckRedirect is copied (not mutated) and given SameOriginRedirects so the
// API key never follows a redirect to another host; a client that already
// sets CheckRedirect is used as is and the caller owns its redirect policy.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = withSameOriginRedirects(httpClient)
		}
	}
}

func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		if baseURL != "" {
			client.baseURL = baseURL
		}
	}
}

func NewClient(apiKey, athleteID string, options ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout:       defaultClientTimeout,
			CheckRedirect: SameOriginRedirects,
		},
		apiKey:    apiKey,
		athleteID: athleteID,
		baseURL:   BaseURL,
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// BaseURL returns the API base URL the client sends requests to.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) Get(resource string, parts []string, query map[string]string, result any) error {
	return c.do(http.MethodGet, resource, parts, query, nil, result)
}

func (c *Client) Put(resource string, parts []string, query map[string]string, body, result any) error {
	return c.do(http.MethodPut, resource, parts, query, body, result)
}

func (c *Client) Post(resource string, parts []string, query map[string]string, body, result any) error {
	return c.do(http.MethodPost, resource, parts, query, body, result)
}

func (c *Client) Delete(resource string, parts []string, query map[string]string, result any) error {
	return c.do(http.MethodDelete, resource, parts, query, nil, result)
}

func (c *Client) Download(resource string, parts []string, query map[string]string) ([]byte, error) {
	path := BuildPath(c.athleteID, resource, parts...)
	url := c.buildFullURL(path, query)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("%w %d: %s", errHTTPStatus, resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading download response: %w", err)
	}

	return data, nil
}

func (c *Client) UploadFile(resource, localPath, filePath string, query map[string]string, result any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for upload: %w", err)
	}

	defer file.Close()

	var body bytes.Buffer
	mpw := multipart.NewWriter(&body)

	part, err := mpw.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copying file data: %w", err)
	}

	if err := mpw.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	pathParts := []string{}
	if localPath != "" {
		pathParts = append(pathParts, localPath)
	}

	path := BuildPath(c.athleteID, resource, pathParts...)
	url := c.buildFullURL(path, query)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, &body)
	if err != nil {
		return fmt.Errorf("creating upload request: %w", err)
	}

	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Content-Type", mpw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("%w %d: %s", errHTTPStatus, resp.StatusCode, string(errBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding upload response: %w", err)
		}
	}

	return nil
}

func (c *Client) do(method, resource string, parts []string, query map[string]string, body, result any) error {
	path := BuildPath(c.athleteID, resource, parts...)
	url := c.buildFullURL(path, query)

	var bodyReader io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}

		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("%w %d: %s", errHTTPStatus, resp.StatusCode, string(errBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("JSON decode failed: %w", err)
		}
	}

	return nil
}

func (c *Client) buildFullURL(path string, query map[string]string) string {
	if len(query) == 0 {
		return c.baseURL + path
	}

	// Always percent-encode and sort query keys, including custom bases
	// (httptest, proxies). Previously only the production BaseURL path encoded.
	var builder strings.Builder
	builder.Grow(len(c.baseURL) + len(path) + 1 + queryEncodedLength(query))
	builder.WriteString(c.baseURL)
	builder.WriteString(path)
	builder.WriteByte('?')
	writeEncodedQuery(&builder, query)

	return builder.String()
}
