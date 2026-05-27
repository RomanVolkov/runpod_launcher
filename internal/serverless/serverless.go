package serverless

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const runpodGraphQLEndpoint = "https://api.runpod.io/graphql"

const (
	DefaultEndpointName = "llm-launcher-serverless"
	DefaultImageName    = "runpod/worker-vllm:stable-cuda12.1.0"
	DefaultWorkersMax   = 3
	DefaultIdleTimeout  = 5
	DefaultScalerType   = "QUEUE_DELAY"
	DefaultScalerValue  = 4
	StatusActive        = "active"
	StatusScaledToZero  = "scaled_to_zero"
)

type Endpoint struct {
	ID         string
	Name       string
	WorkersMin int
	WorkersMax int
}

type Client interface {
	FindEndpointByName(name string) (*Endpoint, error)
	SaveTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error)
	SaveEndpoint(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error)
}

type RunPodServerlessClient struct {
	apiKey     string
	httpClient *http.Client
	BaseURL    string
}

func NewRunPodServerlessClient(apiKey string) Client {
	return &RunPodServerlessClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    runpodGraphQLEndpoint,
	}
}

func (c *RunPodServerlessClient) graphqlRequest(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	if errs, ok := result["errors"].([]interface{}); ok && len(errs) > 0 {
		return nil, fmt.Errorf("graphql error: %v", errs[0])
	}

	return result, nil
}

func (c *RunPodServerlessClient) FindEndpointByName(name string) (*Endpoint, error) {
	query := `
		query ListEndpoints {
			myself {
				endpoints {
					id
					name
					workersMin
					workersMax
				}
			}
		}
	`

	result, err := c.graphqlRequest(query, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: no 'data' field")
	}

	myself, ok := data["myself"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: no 'myself' field")
	}

	endpoints, ok := myself["endpoints"].([]interface{})
	if !ok {
		return nil, nil
	}

	for _, ep := range endpoints {
		epMap, ok := ep.(map[string]interface{})
		if !ok {
			continue
		}

		if stringField(epMap, "name") == name {
			return &Endpoint{
				ID:         stringField(epMap, "id"),
				Name:       stringField(epMap, "name"),
				WorkersMin: intField(epMap, "workersMin"),
				WorkersMax: intField(epMap, "workersMax"),
			}, nil
		}
	}

	return nil, nil
}

func (c *RunPodServerlessClient) SaveTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
	query := `
		mutation SaveTemplate($input: SaveTemplateInput) {
			saveTemplate(input: $input) {
				id
			}
		}
	`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"name":               name,
			"imageName":          imageName,
			"isServerless":       true,
			"containerDiskInGb":  containerDiskGB,
			"volumeInGb":         0,
			"dockerArgs":         "",
			"ports":              "8000/http",
			"env": []map[string]string{
				{"key": "MODEL_NAME", "value": modelName},
				{"key": "HF_TOKEN", "value": apiKey},
			},
		},
	}

	result, err := c.graphqlRequest(query, variables)
	if err != nil {
		return "", err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response structure: no 'data' field")
	}

	saveTemplate, ok := data["saveTemplate"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response structure: no 'saveTemplate' field")
	}

	return stringField(saveTemplate, "id"), nil
}

func (c *RunPodServerlessClient) SaveEndpoint(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
	query := `
		mutation SaveEndpoint($input: EndpointInput) {
			saveEndpoint(input: $input) {
				id
				name
			}
		}
	`

	input := map[string]interface{}{
		"name":        name,
		"gpuIds":      gpuIDs,
		"templateId":  templateID,
		"workersMin":  workersMin,
		"workersMax":  workersMax,
		"idleTimeout": idleTimeout,
		"scalerType":  scalerType,
		"scalerValue": scalerValue,
		"flashboot":   true,
	}

	if endpointID != "" {
		input["id"] = endpointID
	}

	variables := map[string]interface{}{
		"input": input,
	}

	result, err := c.graphqlRequest(query, variables)
	if err != nil {
		return "", err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response structure: no 'data' field")
	}

	saveEndpoint, ok := data["saveEndpoint"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response structure: no 'saveEndpoint' field")
	}

	return stringField(saveEndpoint, "id"), nil
}

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func intField(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
