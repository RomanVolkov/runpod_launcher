// Package pod provides RunPod pod management for runpod-launcher.
//
// # RunPod GraphQL API Reference
//
// Endpoint: POST https://api.runpod.io/graphql
// Content-Type: application/json
// Authorization: Bearer <RunpodAPIKey>
// Body: {"query": "<GraphQL operation string>"}
//
// ## Create pod (on-demand)
//
//	mutation {
//	  podFindAndDeployOnDemand(input: {
//	    gpuTypeId: "AMPERE_16",          // e.g. "AMPERE_16", "ADA_LOVELACE_24"
//	    cloudType: SECURE,               // SECURE or COMMUNITY
//	    imageName: "vllm/vllm-openai:latest",
//	    containerDiskInGb: 50,
//	    volumeMountPath: "/workspace",
//	    dockerArgs: "<bash script>",     // startup command injected via startup.BuildStartupScript
//	    env: [
//	      { key: "LLM_API_KEY",  value: "..." },
//	      { key: "MODEL_NAME",   value: "..." }
//	    ],
//	    ports: "8000/http"
//	  }) {
//	    id
//	    desiredStatus
//	  }
//	}
//
// ## Get pod status
//
//	query {
//	  pod(input: { podId: "<id>" }) {
//	    id
//	    desiredStatus
//	    runtime {
//	      uptimeInSeconds
//	      ports { ip privatePort publicPort type }
//	    }
//	  }
//	}
//
// ## List pods (find by name)
//
//	query {
//	  myself {
//	    pods {
//	      id
//	      name
//	      desiredStatus
//	    }
//	  }
//	}
//
// ## Terminate pod
//
//	mutation {
//	  podTerminate(input: { podId: "<id>" })
//	}
//
// Authorization: API key is sent as "Authorization: Bearer <RunpodAPIKey>" header.
package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/startup"
)

// PodStatus holds status information about a RunPod pod.
type PodStatus struct {
	ID            string
	Status        string
	DesiredStatus string
}

// PodClient is the interface for interacting with RunPod's pod management API.
// It is designed for easy test mocking without external HTTP.
type PodClient interface {
	// CreatePod provisions a new pod according to cfg and llmAPIKey, returning the pod ID.
	CreatePod(cfg *config.Config, llmAPIKey string) (string, error)

	// GetPodStatus returns the current status of the pod with the given ID.
	GetPodStatus(podID string) (*PodStatus, error)

	// TerminatePod terminates the pod with the given ID.
	TerminatePod(podID string) error

	// FindPodByName returns the pod ID of a running pod with the given name,
	// or ("", nil) if no such pod is found.
	FindPodByName(name string) (string, error)
}

const runpodGraphQLEndpoint = "https://api.runpod.io/graphql"

// DefaultPodName is the pod name used when config.PodName is empty.
const DefaultPodName = "llm-launcher"

// DefaultServicePort is the port on which the vLLM service listens inside the pod.
// It is used both when building the startup script and when registering the pod's
// HTTP port mapping, so both must stay in sync via this constant.
const DefaultServicePort = 8000

// DefaultImageName is the container image used when config.ImageName is empty.
const DefaultImageName = "vllm/vllm-openai:latest"

// DefaultContainerDiskGB is the container disk size (in GB) used when config.ContainerDiskGB is zero.
const DefaultContainerDiskGB = 50

// DefaultVolumeMountPath is the volume mount path used when config.VolumeMountPath is empty.
const DefaultVolumeMountPath = "/workspace"

// StatusNotFound is the sentinel status emitted by the status command when no pod exists.
// Note on status string conventions:
//   - up/down commands emit synthetic CLI-friendly labels: "running" and "terminated".
//   - status command emits raw RunPod desiredStatus values (e.g. "RUNNING", "STARTING")
//     plus this sentinel when no pod is found.
//
// These two tiers are intentional: up/down report the outcome of their action in a
// simple boolean sense; status reflects the live RunPod API state verbatim.
const StatusNotFound = "not_found"

// StatusRunning is the synthetic label emitted by the up command on success.
const StatusRunning = "running"

// StatusTerminated is the synthetic label emitted by the down command on success.
const StatusTerminated = "terminated"

// RunPodClient implements PodClient by calling RunPod's GraphQL API over net/http.
type RunPodClient struct {
	apiKey     string
	httpClient *http.Client
	// baseURL is the GraphQL endpoint. Defaults to runpodGraphQLEndpoint.
	// Tests override this to point at a local httptest.Server.
	baseURL string
}

// NewRunPodClient returns a new RunPodClient authenticated with the given API key.
func NewRunPodClient(apiKey string) PodClient {
	return &RunPodClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    runpodGraphQLEndpoint,
	}
}

// graphqlRequest sends a GraphQL request to the RunPod API and decodes the response body.
func (c *RunPodClient) graphqlRequest(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RunPod API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RunPod API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RunPod API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode RunPod API response: %w", err)
	}

	// Surface GraphQL-level errors.
	if errs, ok := result["errors"]; ok {
		return nil, fmt.Errorf("RunPod GraphQL error: %v", errs)
	}

	return result, nil
}

// CreatePod provisions a new RunPod pod described by cfg and returns its pod ID.
// It injects the provided llmAPIKey and other configuration as pod env vars and uses
// startup.BuildStartupScript as the startup command.
func (c *RunPodClient) CreatePod(cfg *config.Config, llmAPIKey string) (string, error) {
	startupScript, err := startup.BuildStartupScript(cfg.ModelName, llmAPIKey, DefaultServicePort, cfg.MaxModelLen, cfg.ToolCallParser)
	if err != nil {
		return "", fmt.Errorf("failed to build startup script: %w", err)
	}

	// Build the env array for the GraphQL mutation.
	// These env vars are passed to the pod.
	envVars := []map[string]string{}
	for k, v := range cfg.EnvVars {
		envVars = append(envVars, map[string]string{"key": k, "value": v})
	}

	// For Ollama, set OLLAMA_HOST to listen on all interfaces and the specified port
	if cfg.ImageName != "" && strings.Contains(strings.ToLower(cfg.ImageName), "ollama") {
		envVars = append(envVars, map[string]string{
			"key":   "OLLAMA_HOST",
			"value": fmt.Sprintf("0.0.0.0:%d", DefaultServicePort),
		})
	}

	imageName := cfg.ImageName
	if imageName == "" {
		imageName = DefaultImageName
	}
	diskGB := cfg.ContainerDiskGB
	if diskGB == 0 {
		diskGB = DefaultContainerDiskGB
	}
	volumePath := cfg.VolumeMountPath
	if volumePath == "" {
		volumePath = DefaultVolumeMountPath
	}
	podName := cfg.PodName
	if podName == "" {
		podName = DefaultPodName
	}

	query := `
mutation CreatePod($input: PodFindAndDeployOnDemandInput!) {
  podFindAndDeployOnDemand(input: $input) {
    id
    desiredStatus
  }
}`

	input := map[string]interface{}{
		"gpuTypeId": cfg.GPUTypeID,
		"gpuCount":  1,
		// cloudType is fixed to SECURE (vs COMMUNITY) to ensure dedicated GPU
		// resources and avoid shared-host networking restrictions. There is
		// intentionally no config option for this.
		"cloudType":        "SECURE",
		"name":             podName,
		"imageName":        imageName,
		"containerDiskInGb": diskGB,
		"volumeMountPath":  volumePath,
		"dockerArgs":       startupScript,
		"env":              envVars,
		"startSsh":         true,
		"ports":            "8000/http",
	}

	// Add CUDA version constraint if specified
	if cfg.CudaVersion != "" {
		input["cudaVersion"] = cfg.CudaVersion
	}

	// Add region preference if specified
	if cfg.Region != "" {
		input["region"] = cfg.Region
	}

	// Log the request for debugging
	inputJSON, _ := json.MarshalIndent(input, "", "  ")
	fmt.Fprintf(os.Stderr, "Creating pod with input:\n%s\n", string(inputJSON))

	result, err := c.graphqlRequest(query, map[string]interface{}{"input": input})
	if err != nil {
		return "", err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected RunPod API response shape: %v", result)
	}
	podData, ok := data["podFindAndDeployOnDemand"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("podFindAndDeployOnDemand not found in response: %v", data)
	}
	id, ok := podData["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("pod ID missing in RunPod response: %v", podData)
	}
	return id, nil
}

// GetPodStatus returns the current status of the pod identified by podID.
func (c *RunPodClient) GetPodStatus(podID string) (*PodStatus, error) {
	query := `
query GetPod($input: PodFilter!) {
  pod(input: $input) {
    id
    desiredStatus
    runtime {
      uptimeInSeconds
    }
  }
}`

	result, err := c.graphqlRequest(query, map[string]interface{}{
		"input": map[string]string{"podId": podID},
	})
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected RunPod API response: %v", result)
	}

	// A null pod field ({"data":{"pod":null}}) means the pod does not exist;
	// return a not-found sentinel rather than an error so callers can distinguish
	// "pod gone" from a real API failure.
	if data["pod"] == nil {
		return &PodStatus{ID: podID, Status: "NOT_FOUND", DesiredStatus: ""}, nil
	}

	podData, ok := data["pod"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected pod field type in RunPod API response: %v", data)
	}

	status := &PodStatus{
		ID:            podID,
		DesiredStatus: stringField(podData, "desiredStatus"),
	}

	// Report RUNNING only when runtime is non-nil AND desiredStatus is explicitly
	// "RUNNING". This guards against a blank desiredStatus (API parse gap) or
	// "EXITED" (pod being torn down still briefly has a runtime object).
	if podData["runtime"] != nil && status.DesiredStatus == "RUNNING" {
		status.Status = "RUNNING"
	} else {
		status.Status = status.DesiredStatus
	}

	return status, nil
}

// TerminatePod terminates the pod with the given ID.
func (c *RunPodClient) TerminatePod(podID string) error {
	query := `
mutation TerminatePod($input: PodTerminateInput!) {
  podTerminate(input: $input)
}`

	_, err := c.graphqlRequest(query, map[string]interface{}{
		"input": map[string]string{"podId": podID},
	})
	return err
}

// FindPodByName returns the pod ID of an existing pod with the given name,
// or ("", nil) if no matching pod is found.
func (c *RunPodClient) FindPodByName(name string) (string, error) {
	query := `
query ListPods {
  myself {
    pods {
      id
      name
      desiredStatus
    }
  }
}`

	result, err := c.graphqlRequest(query, nil)
	if err != nil {
		return "", err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected RunPod API response: %v", result)
	}
	myself, ok := data["myself"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("myself not found in response: %v", data)
	}
	// A null pods field is a valid "no pods" response; any other non-array type
	// indicates an unexpected API shape and should surface as an error.
	if myself["pods"] == nil {
		return "", nil
	}
	pods, ok := myself["pods"].([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected pods field type in RunPod API response: %v", myself)
	}

	for _, p := range pods {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if stringField(pod, "name") == name && stringField(pod, "desiredStatus") != "EXITED" {
			return stringField(pod, "id"), nil
		}
	}
	return "", nil
}

// stringField safely extracts a string field from a map.
func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// WaitForReady polls GetPodStatus every tickInterval until the pod status is "RUNNING"
// or timeout is exceeded. Progress dots are printed to stderr to keep stdout clean
// for --json output mode.
//
// tickInterval controls the polling frequency; pass 0 to use the default of 5 seconds.
func WaitForReady(client PodClient, podID string, timeout time.Duration, stderr io.Writer, tickInterval ...time.Duration) error {
	interval := 5 * time.Second
	if len(tickInterval) > 0 && tickInterval[0] > 0 {
		interval = tickInterval[0]
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately before waiting for the first tick.
	status, err := client.GetPodStatus(podID)
	if err == nil && status.Status == "RUNNING" {
		return nil
	}
	fmt.Fprint(stderr, ".")

	for {
		select {
		case <-timer.C:
			fmt.Fprintln(stderr) // newline after dots
			return fmt.Errorf("timed out waiting for pod %s to become RUNNING after %s", podID, timeout)
		case <-ticker.C:
			status, err := client.GetPodStatus(podID)
			if err != nil {
				fmt.Fprint(stderr, "e")
				continue
			}
			if status.Status == "RUNNING" {
				fmt.Fprintln(stderr) // newline after dots
				return nil
			}
			fmt.Fprint(stderr, ".")
		}
	}
}

// CheckModelStatus queries the vLLM API to check if a model is loaded and ready.
// baseURL should be the vLLM endpoint (e.g., "https://...-8000.proxy.runpod.net/v1").
// apiKey is the API key required by the vLLM server (can be empty if no auth is required).
// Returns true if the model is loaded, false otherwise.
func CheckModelStatus(baseURL, modelName, apiKey string) (bool, error) {
	// Query the /models endpoint to list loaded models
	// vLLM's OpenAI-compatible API serves models at {baseURL}/models
	modelsURL := baseURL + "/models"

	req, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key header if provided (vLLM requires it if --api-key is set)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to query API at %s: %w", modelsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API at %s returned HTTP %d: %s", modelsURL, resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	// Log the response for debugging
	fmt.Fprintf(os.Stderr, "Models response: %s\n", string(respBody))

	// Parse the response to check if the model is in the list
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	// OpenAI-compatible API returns {"data": [{"id": "model-name"}, ...]}
	data, ok := result["data"].([]interface{})
	if !ok {
		return false, fmt.Errorf("unexpected response format: no 'data' field")
	}

	for _, item := range data {
		model, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if modelID, ok := model["id"].(string); ok {
			// Check exact match
			if modelID == modelName {
				return true, nil
			}
			// For Ollama: if modelName is "gemma4", also match "gemma4:latest"
			if modelID == modelName+":latest" {
				return true, nil
			}
		}
	}

	return false, nil
}

// PullOllamaModel pulls a model in Ollama via the /api/pull endpoint.
// It retries with backoff to wait for Ollama server to be ready.
func PullOllamaModel(baseURL, modelName string, stderr io.Writer) error {
	pullURL := baseURL + "/api/pull"

	payload := map[string]interface{}{
		"name":   modelName,
		"stream": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal pull request: %w", err)
	}

	// Retry with backoff to wait for Ollama server to start
	maxRetries := 12
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPost, pullURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create pull request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Minute} // Long timeout for model download
		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				fmt.Fprintf(stderr, ".")
				time.Sleep(time.Duration((attempt+1)*5) * time.Second)
				continue
			}
			return fmt.Errorf("failed to pull model from Ollama after %d retries: %w", maxRetries, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Fprintf(stderr, " pulled\n")
			return nil
		}

		// If server not ready, retry
		if resp.StatusCode >= 500 || resp.StatusCode == 404 {
			if attempt < maxRetries-1 {
				fmt.Fprintf(stderr, ".")
				time.Sleep(time.Duration((attempt+1)*5) * time.Second)
				continue
			}
		}

		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama pull failed with HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return fmt.Errorf("Ollama pull timeout after %d retries", maxRetries)
}

// WaitForModelReady polls CheckModelStatus until the model is loaded or timeout is reached.
// It writes progress dots to stderr as it waits.
// tickInterval is the polling interval; if zero or omitted, defaults to 5 seconds.
func WaitForModelReady(baseURL, modelName, apiKey string, timeout time.Duration, stderr io.Writer, tickInterval ...time.Duration) error {
	interval := 5 * time.Second
	if len(tickInterval) > 0 && tickInterval[0] > 0 {
		interval = tickInterval[0]
	}

	deadline := time.Now().Add(timeout)
	for {
		isLoaded, err := CheckModelStatus(baseURL, modelName, apiKey)
		if err == nil && isLoaded {
			fmt.Fprintf(stderr, " ready\n")
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("model %q did not load within %v", modelName, timeout)
		}

		fmt.Fprint(stderr, ".")
		time.Sleep(interval)
	}
}
