package serverless

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func (c *RunPodServerlessClient) findTemplateByName(name string) (*map[string]json.RawMessage, error) {
	out, err := RunpodCtlFn(c.apiKey, "template", "list", "--type", "user")
	if err != nil {
		return nil, fmt.Errorf("runpodctl template list: %w\n%s", err, out)
	}

	var templates []map[string]json.RawMessage
	if err := json.Unmarshal(out, &templates); err != nil {
		return nil, fmt.Errorf("failed to parse template list output: %w\n%s", err, out)
	}

	for i, tpl := range templates {
		var tplName string
		if err := json.Unmarshal(tpl["name"], &tplName); err != nil {
			continue
		}
		if tplName != name {
			continue
		}
		return &templates[i], nil
	}

	return nil, nil
}

func (c *RunPodServerlessClient) CreateTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
	// Check if template exists and has correct MODEL_NAME
	existing, err := c.findTemplateByName(name)
	if err != nil {
		return "", err
	}

	if existing != nil {
		// Template exists; check if it has the correct image and MODEL_NAME
		matchesImage := false
		matchesModel := false

		if imgRaw, ok := (*existing)["imageName"]; ok {
			var currentImage string
			if err := json.Unmarshal(imgRaw, &currentImage); err == nil {
				matchesImage = currentImage == imageName
			}
		}

		if envRaw, ok := (*existing)["env"]; ok {
			var envMap map[string]string
			if err := json.Unmarshal(envRaw, &envMap); err == nil {
				if currentModel, hasModel := envMap["MODEL_NAME"]; hasModel {
					matchesModel = currentModel == modelName
				}
			}
		}

		if matchesImage && matchesModel {
			// Template already has correct image and MODEL_NAME; reuse it
			var id string
			if err := json.Unmarshal((*existing)["id"], &id); err == nil {
				return id, nil
			}
		}

		// Template exists but has wrong image or MODEL_NAME; delete and recreate
		var id string
		if err := json.Unmarshal((*existing)["id"], &id); err == nil {
			_, _ = RunpodCtlFn(c.apiKey, "template", "delete", id)
		}
	}

	// Build environment variables based on image type
	env := map[string]string{}
	if isOllamaImage(imageName) {
		// Ollama-specific: listen on port 8000 (serverless expects port 8000)
		// Format must include protocol: http://host:port
		env["OLLAMA_HOST"] = "http://0.0.0.0:8000"
		// Set model to pull (format: modelname:tag, e.g. gemma4:latest)
		env["OLLAMA_MODEL"] = modelName
	} else {
		// vLLM or other frameworks: use MODEL_NAME format (HuggingFace repo IDs)
		env["MODEL_NAME"] = modelName
		env["HF_TOKEN"] = apiKey
	}

	envJSON, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("failed to marshal env vars: %w", err)
	}

	args := []string{
		"template", "create",
		"--name", name,
		"--image", imageName,
		"--serverless",
		"--container-disk-in-gb", fmt.Sprintf("%d", containerDiskGB),
		"--ports", "8000/http",
		"--env", string(envJSON),
	}

	// For Ollama, add startup script to:
	// 1. Start socat reverse proxy (8000 → 11434)
	// 2. Start Ollama
	// 3. Pull the model so it's ready when requests come in
	if isOllamaImage(imageName) {
		startCmd := fmt.Sprintf(
			"socat TCP-LISTEN:8000,reuseaddr,fork TCP:localhost:11434 & sleep 1 && /bin/ollama serve & OLLAMA_PID=$! && sleep 3 && /bin/ollama pull %s && wait $OLLAMA_PID",
			modelName,
		)
		args = append(args,
			"--docker-start-cmd", startCmd,
		)
	}

	out, err := RunpodCtlFn(c.apiKey, args...)
	if err != nil {
		return "", fmt.Errorf("runpodctl template create: %w\n%s", err, out)
	}

	return parseID(out)
}

func isOllamaImage(imageName string) bool {
	return strings.Contains(imageName, "ollama")
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
