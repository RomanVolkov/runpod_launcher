package serverless

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

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
	CreateTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error)
	CreateEndpoint(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error)
	ScaleToZero(endpointID string) error
	DeleteEndpoint(endpointID string) error
}

// RunpodCtlFn is the function used to call runpodctl. Tests override this.
var RunpodCtlFn = func(apiKey string, args ...string) ([]byte, error) {
	cmd := exec.Command("runpodctl", args...)
	cmd.Env = append(os.Environ(), "RUNPOD_API_KEY="+apiKey)
	return cmd.CombinedOutput()
}

type RunPodServerlessClient struct {
	apiKey string
}

func NewRunPodServerlessClient(apiKey string) Client {
	return &RunPodServerlessClient{apiKey: apiKey}
}

func (c *RunPodServerlessClient) FindEndpointByName(name string) (*Endpoint, error) {
	out, err := RunpodCtlFn(c.apiKey, "serverless", "list")
	if err != nil {
		return nil, fmt.Errorf("runpodctl serverless list: %w", err)
	}

	var endpoints []map[string]json.RawMessage
	if err := json.Unmarshal(out, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse serverless list output: %w\n%s", err, out)
	}

	for _, ep := range endpoints {
		var epName string
		if err := json.Unmarshal(ep["name"], &epName); err != nil {
			continue
		}
		if epName != name {
			continue
		}
		var id string
		if err := json.Unmarshal(ep["id"], &id); err != nil {
			continue
		}
		result := &Endpoint{ID: id, Name: epName}
		if raw, ok := ep["workersMin"]; ok {
			var v int
			if err := json.Unmarshal(raw, &v); err == nil {
				result.WorkersMin = v
			}
		}
		if raw, ok := ep["workersMax"]; ok {
			var v int
			if err := json.Unmarshal(raw, &v); err == nil {
				result.WorkersMax = v
			}
		}
		return result, nil
	}

	return nil, nil
}

func (c *RunPodServerlessClient) CreateTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
	envJSON, err := json.Marshal(map[string]string{
		"MODEL_NAME": modelName,
		"HF_TOKEN":   apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal env vars: %w", err)
	}

	out, err := RunpodCtlFn(c.apiKey,
		"template", "create",
		"--name", name,
		"--image", imageName,
		"--serverless",
		"--container-disk-in-gb", fmt.Sprintf("%d", containerDiskGB),
		"--ports", "8000/http",
		"--env", string(envJSON),
	)
	if err != nil {
		return "", fmt.Errorf("runpodctl template create: %w\n%s", err, out)
	}

	return parseID(out)
}

func (c *RunPodServerlessClient) CreateEndpoint(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
	scaleBy := "delay"
	if scalerType == "REQUEST_COUNT" {
		scaleBy = "requests"
	}

	out, err := RunpodCtlFn(c.apiKey,
		"serverless", "create",
		"--name", name,
		"--template-id", templateID,
		"--gpu-id", gpuID,
		"--workers-min", "0",
		"--workers-max", fmt.Sprintf("%d", workersMax),
		"--idle-timeout", fmt.Sprintf("%d", idleTimeout),
		"--scale-by", scaleBy,
		"--scale-threshold", fmt.Sprintf("%d", scalerValue),
		"--flash-boot",
	)
	if err != nil {
		return "", fmt.Errorf("runpodctl serverless create: %w\n%s", err, out)
	}

	return parseID(out)
}

func (c *RunPodServerlessClient) ScaleToZero(endpointID string) error {
	out, err := RunpodCtlFn(c.apiKey,
		"serverless", "update", endpointID,
		"--workers-min", "0",
		"--workers-max", "0",
	)
	if err != nil {
		return fmt.Errorf("runpodctl serverless update: %w\n%s", err, out)
	}
	return nil
}

func (c *RunPodServerlessClient) DeleteEndpoint(endpointID string) error {
	out, err := RunpodCtlFn(c.apiKey, "serverless", "delete", endpointID)
	if err != nil {
		return fmt.Errorf("runpodctl serverless delete: %w\n%s", err, out)
	}
	return nil
}

func parseID(out []byte) (string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		return "", fmt.Errorf("failed to parse CLI output: %w\n%s", err, out)
	}
	raw, ok := obj["id"]
	if !ok {
		return "", fmt.Errorf("no 'id' field in CLI output: %s", out)
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		return "", fmt.Errorf("failed to parse 'id' field: %w", err)
	}
	return id, nil
}
