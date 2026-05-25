package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	athleteID  string
	baseURL    string
}

func NewClient(apiKey, athleteID string) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		athleteID:  athleteID,
		baseURL:    BaseURL,
	}
}

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

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) UploadFile(resource, localPath, filePath string, query map[string]string, result any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	w.Close()

	pathParts := []string{}
	if localPath != "" {
		pathParts = append(pathParts, localPath)
	}
	path := BuildPath(c.athleteID, resource, pathParts...)
	url := c.buildFullURL(path, query)

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
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
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", BuildAuthHeader(c.apiKey))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("JSON decode failed: %w", err)
		}
	}
	return nil
}

func (c *Client) buildFullURL(path string, query map[string]string) string {
	if c.baseURL != BaseURL {
		if len(query) == 0 {
			return c.baseURL + path
		}
		q := ""
		for k, v := range query {
			if q != "" {
				q += "&"
			}
			q += k + "=" + v
		}
		return c.baseURL + path + "?" + q
	}
	return BuildURL(path, query)
}
