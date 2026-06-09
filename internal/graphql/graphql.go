package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultGraphQLURL = "https://api.runpod.io/graphql"
	defaultTimeout    = 30 * time.Second
)

// Client is a GraphQL client for RunPod API
type Client struct {
	url        string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new GraphQL client with the given API key
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key required")
	}

	return &Client{
		url:        DefaultGraphQLURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}, nil
}

// NewClientWithURL creates a new GraphQL client with a custom URL
func NewClientWithURL(apiKey, url string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key required")
	}
	if url == "" {
		url = DefaultGraphQLURL
	}

	return &Client{
		url:        url,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}, nil
}

// Query executes a GraphQL query and returns the raw response bytes
func (c *Client) Query(input GraphQLInput) ([]byte, error) {
	if input.Variables == nil {
		input.Variables = map[string]interface{}{}
	}

	jsonValue, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphql request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("graphql error: status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetGPUTypes queries all available GPU types
func (c *Client) GetGPUTypes() ([]GPUTypeInfo, error) {
	input := GraphQLInput{
		Query: `
		query {
		  gpuTypes {
			id
			displayName
			memoryInGb
			secureCloud
			communityCloud
		  }
		}
		`,
	}

	body, err := c.Query(input)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			GPUTypes []GPUTypeInfo `json:"gpuTypes"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse gpuTypes response: %w", err)
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", response.Errors[0].Message)
	}

	return response.Data.GPUTypes, nil
}

// GetLowestPrice queries GPU pricing with filters
// Note: RunPod's public GraphQL API does not expose pricing data.
// This method returns empty pricing and is kept for future API compatibility.
func (c *Client) GetLowestPrice(input *GPULowestPriceInput) ([]GPUPriceInfo, error) {
	if input == nil {
		return nil, fmt.Errorf("input required")
	}

	// Pricing is not available through the public GraphQL API
	// Return empty list so callers can gracefully show N/A
	return []GPUPriceInfo{}, nil
}

// CreatePod creates a pod on-demand using the podFindAndDeployOnDemand mutation
func (c *Client) CreatePod(input *PodFindAndDeployInput) (*PodInfo, error) {
	if input == nil {
		return nil, fmt.Errorf("input required")
	}

	gqlInput := GraphQLInput{
		Query: `
		mutation CreatePod($input: PodFindAndDeployOnDemandInput!) {
		  podFindAndDeployOnDemand(input: $input) {
			id
			name
			desiredStatus
			costPerHr
			imageName
			containerDiskInGb
		  }
		}
		`,
		Variables: map[string]interface{}{"input": input},
	}

	body, err := c.Query(gqlInput)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			Pod *PodInfo `json:"podFindAndDeployOnDemand"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse pod creation response: %w", err)
	}

	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", response.Errors[0].Message)
	}

	if response.Data.Pod == nil {
		return nil, fmt.Errorf("pod creation returned empty response")
	}

	return response.Data.Pod, nil
}
