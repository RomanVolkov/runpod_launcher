package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient fetches model specs from a running Ollama instance
type OllamaClient struct {
	url    string
	client *http.Client
}

// OllamaModel represents model info from Ollama API
type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

// OllamaTagsResponse is the response from /api/tags
type OllamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// NewOllamaClient creates a client for local/remote Ollama instance
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		url: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchModels retrieves available models from Ollama
func (c *OllamaClient) FetchModels() ([]OllamaModel, error) {
	url := c.url + "/api/tags"
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Ollama models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error: status %d: %s", resp.StatusCode, string(body))
	}

	var result OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	return result.Models, nil
}

// GetModelNames returns a list of available model names from Ollama
func (c *OllamaClient) GetModelNames() ([]string, error) {
	models, err := c.FetchModels()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, m := range models {
		names = append(names, m.Name)
	}
	return names, nil
}
