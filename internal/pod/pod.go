package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

// GPUType holds information about a GPU type on RunPod.
type GPUType struct {
	ID                        string
	DisplayName               string
	MemoryInGb                int
	SecurePrice               float64
	CommunityPrice            float64
	SecureSpotPrice           float64
	CommunitySpotPrice        float64
	SecureCloud               bool
	CommunityCloud            bool
	MaxGpuCountSecureCloud    int
	MaxGpuCountCommunityCloud int
}

// PodClient is the interface for interacting with RunPod's pod management API.
type PodClient interface {
	CreatePod(cfg *config.Config, llmAPIKey string) (string, error)
	GetPodStatus(podID string) (*PodStatus, error)
	TerminatePod(podID string) error
	FindPodByName(name string) (string, error)
	GetGPUTypes() ([]GPUType, error)
}

const DefaultPodName = "llm-launcher"
const DefaultServicePort = 8000
const DefaultImageName = "vllm/vllm-openai:latest"
const DefaultContainerDiskGB = 50
const DefaultVolumeMountPath = "/workspace"

const StatusNotFound = "not_found"
const StatusRunning = "running"
const StatusTerminated = "terminated"

// RunpodCtlFn is the function used to call runpodctl. Tests override this.
var RunpodCtlFn = func(apiKey string, args ...string) ([]byte, error) {
	cmd := exec.Command("runpodctl", args...)
	cmd.Env = append(os.Environ(), "RUNPOD_API_KEY="+apiKey)
	return cmd.CombinedOutput()
}

// GetOllamaModelContextFunc is injected for testing; default calls GetOllamaModelContext.
var GetOllamaModelContextFunc = GetOllamaModelContext

// WaitForModelReadyFunc is injected for testing; default calls WaitForModelReady.
var WaitForModelReadyFunc = WaitForModelReady

// RunPodClient implements PodClient using the runpodctl CLI.
type RunPodClient struct {
	apiKey string
}

// NewRunPodClient returns a new RunPodClient authenticated with the given API key.
func NewRunPodClient(apiKey string) PodClient {
	return &RunPodClient{apiKey: apiKey}
}

func (c *RunPodClient) CreatePod(cfg *config.Config, llmAPIKey string) (string, error) {
	startupScript, err := startup.BuildStartupScript(cfg.ModelName, llmAPIKey, DefaultServicePort, cfg.MaxModelLen, cfg.ToolCallParser)
	if err != nil {
		return "", fmt.Errorf("failed to build startup script: %w", err)
	}

	envMap := make(map[string]string)
	for k, v := range cfg.EnvVars {
		envMap[k] = v
	}

	if cfg.ImageName != "" && strings.Contains(strings.ToLower(cfg.ImageName), "ollama") {
		envMap["OLLAMA_HOST"] = fmt.Sprintf("0.0.0.0:%d", DefaultServicePort)

		if cfg.OllamaContextLen > 0 {
			fmt.Fprintf(os.Stderr, "Using Ollama context from config: %d tokens\n", cfg.OllamaContextLen)
			envMap["OLLAMA_CONTEXT_LENGTH"] = fmt.Sprintf("%d", cfg.OllamaContextLen)
		} else {
			fmt.Fprintf(os.Stderr, "Looking up context window for model: %s\n", cfg.ModelName)
			detected, err := GetOllamaModelContextFunc(cfg.ModelName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not look up context window: %v\n", err)
			} else if detected > 0 {
				fmt.Fprintf(os.Stderr, "Auto-detected context window: %d tokens\n", detected)
				envMap["OLLAMA_CONTEXT_LENGTH"] = fmt.Sprintf("%d", detected)
			} else {
				fmt.Fprintf(os.Stderr, "warning: model %q not recognized in context mapping - using Ollama default (2048 tokens)\n", cfg.ModelName)
			}
		}
	}

	envJSON, err := json.Marshal(envMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal env vars: %w", err)
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

	args := []string{
		"pod", "create",
		"--name", podName,
		"--image", imageName,
		"--gpu-id", cfg.GPUTypeID,
		"--cloud-type", "SECURE",
		"--container-disk-in-gb", fmt.Sprintf("%d", diskGB),
		"--volume-mount-path", volumePath,
		"--ports", "8000/http",
		"--env", string(envJSON),
		"--docker-args", startupScript,
		"--ssh",
	}

	if cfg.CudaVersion != "" {
		args = append(args, "--min-cuda-version", cfg.CudaVersion)
	}
	if cfg.Region != "" {
		args = append(args, "--data-center-ids", cfg.Region)
	}

	inputJSON, _ := json.MarshalIndent(map[string]interface{}{
		"podName":    podName,
		"imageName":  imageName,
		"gpuTypeId":  cfg.GPUTypeID,
		"diskGB":     diskGB,
		"volumePath": volumePath,
		"env":        envMap,
	}, "", "  ")
	fmt.Fprintf(os.Stderr, "Creating pod with input:\n%s\n", string(inputJSON))

	out, err := RunpodCtlFn(c.apiKey, args...)
	if err != nil {
		return "", fmt.Errorf("runpodctl pod create: %w\n%s", err, out)
	}

	return parsePodID(out)
}

func (c *RunPodClient) GetPodStatus(podID string) (*PodStatus, error) {
	out, err := RunpodCtlFn(c.apiKey, "pod", "get", podID)
	if err != nil {
		return nil, fmt.Errorf("runpodctl pod get: %w\n%s", err, out)
	}

	var pod map[string]json.RawMessage
	if err := json.Unmarshal(out, &pod); err != nil {
		return nil, fmt.Errorf("failed to parse pod get output: %w\n%s", err, out)
	}

	if pod == nil {
		return &PodStatus{ID: podID, Status: "NOT_FOUND", DesiredStatus: ""}, nil
	}

	var desiredStatus string
	if raw, ok := pod["desiredStatus"]; ok {
		_ = json.Unmarshal(raw, &desiredStatus)
	}

	status := &PodStatus{
		ID:            podID,
		DesiredStatus: desiredStatus,
	}

	hasRuntime := pod["runtime"] != nil
	rawRuntime, _ := pod["runtime"]
	if rawRuntime != nil {
		var runtimeVal interface{}
		if err := json.Unmarshal(rawRuntime, &runtimeVal); err == nil && runtimeVal != nil {
			hasRuntime = true
		} else {
			hasRuntime = false
		}
	}

	if hasRuntime && desiredStatus == "RUNNING" {
		status.Status = "RUNNING"
	} else {
		status.Status = desiredStatus
	}

	return status, nil
}

func (c *RunPodClient) TerminatePod(podID string) error {
	out, err := RunpodCtlFn(c.apiKey, "pod", "delete", podID)
	if err != nil {
		return fmt.Errorf("runpodctl pod delete: %w\n%s", err, out)
	}
	return nil
}

func (c *RunPodClient) FindPodByName(name string) (string, error) {
	out, err := RunpodCtlFn(c.apiKey, "pod", "list", "--name", name)
	if err != nil {
		return "", fmt.Errorf("runpodctl pod list: %w\n%s", err, out)
	}

	var pods []map[string]json.RawMessage
	if err := json.Unmarshal(out, &pods); err != nil {
		return "", fmt.Errorf("failed to parse pod list output: %w\n%s", err, out)
	}

	for _, pod := range pods {
		var podName, desiredStatus, id string
		if raw, ok := pod["name"]; ok {
			_ = json.Unmarshal(raw, &podName)
		}
		if raw, ok := pod["desiredStatus"]; ok {
			_ = json.Unmarshal(raw, &desiredStatus)
		}
		if raw, ok := pod["id"]; ok {
			_ = json.Unmarshal(raw, &id)
		}

		if podName == name && desiredStatus != "EXITED" {
			return id, nil
		}
	}

	return "", nil
}

func (c *RunPodClient) GetGPUTypes() ([]GPUType, error) {
	out, err := RunpodCtlFn(c.apiKey, "gpu", "list")
	if err != nil {
		return nil, fmt.Errorf("runpodctl gpu list: %w\n%s", err, out)
	}

	var rawGPUs []map[string]json.RawMessage
	if err := json.Unmarshal(out, &rawGPUs); err != nil {
		return nil, fmt.Errorf("failed to parse gpu list output: %w\n%s", err, out)
	}

	var gpuTypes []GPUType
	for _, raw := range rawGPUs {
		gpu := GPUType{}
		if v, ok := raw["id"]; ok {
			_ = json.Unmarshal(v, &gpu.ID)
		}
		if v, ok := raw["displayName"]; ok {
			_ = json.Unmarshal(v, &gpu.DisplayName)
		}
		if v, ok := raw["memoryInGb"]; ok {
			_ = json.Unmarshal(v, &gpu.MemoryInGb)
		}
		if v, ok := raw["securePrice"]; ok {
			_ = json.Unmarshal(v, &gpu.SecurePrice)
		}
		if v, ok := raw["communityPrice"]; ok {
			_ = json.Unmarshal(v, &gpu.CommunityPrice)
		}
		if v, ok := raw["secureSpotPrice"]; ok {
			_ = json.Unmarshal(v, &gpu.SecureSpotPrice)
		}
		if v, ok := raw["communitySpotPrice"]; ok {
			_ = json.Unmarshal(v, &gpu.CommunitySpotPrice)
		}
		if v, ok := raw["secureCloud"]; ok {
			_ = json.Unmarshal(v, &gpu.SecureCloud)
		}
		if v, ok := raw["communityCloud"]; ok {
			_ = json.Unmarshal(v, &gpu.CommunityCloud)
		}
		if v, ok := raw["maxGpuCountSecureCloud"]; ok {
			_ = json.Unmarshal(v, &gpu.MaxGpuCountSecureCloud)
		}
		if v, ok := raw["maxGpuCountCommunityCloud"]; ok {
			_ = json.Unmarshal(v, &gpu.MaxGpuCountCommunityCloud)
		}
		gpuTypes = append(gpuTypes, gpu)
	}

	return gpuTypes, nil
}

func parsePodID(out []byte) (string, error) {
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

// GetOllamaModelContext returns the maximum context window (in tokens) for a given Ollama model.
func GetOllamaModelContext(modelName string) (int, error) {
	lower := strings.ToLower(modelName)

	baseName := lower
	if idx := strings.Index(lower, ":"); idx != -1 {
		baseName = lower[:idx]
	}
	baseName = strings.ReplaceAll(baseName, ".", "")

	modelContextMap := map[string]int{
		"gemma":       9216,
		"gemma2":      9216,
		"gemma4":      262144,
		"mistral":     32768,
		"mixtral":     32768,
		"llama":       4096,
		"llama2":      4096,
		"llama3":      8192,
		"llama31":     131072,
		"qwen":        32768,
		"qwen36":      32768,  // Qwen 3.6 models (e.g. qwen3.6:27b)
		"neural-chat": 4096,
		"zephyr":      4096,
		"openchat":    8192,
		"starling":    4096,
	}

	if ctx, ok := modelContextMap[baseName]; ok {
		return ctx, nil
	}

	return 0, nil
}

// WaitForReady polls GetPodStatus every tickInterval until the pod status is "RUNNING"
// or timeout is exceeded. Progress dots are printed to stderr to keep stdout clean
// for --json output mode.
func WaitForReady(client PodClient, podID string, timeout time.Duration, stderr io.Writer, tickInterval ...time.Duration) error {
	interval := 5 * time.Second
	if len(tickInterval) > 0 && tickInterval[0] > 0 {
		interval = tickInterval[0]
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	status, err := client.GetPodStatus(podID)
	if err == nil && status.Status == "RUNNING" {
		return nil
	}
	fmt.Fprint(stderr, ".")

	for {
		select {
		case <-timer.C:
			fmt.Fprintln(stderr)
			return fmt.Errorf("timed out waiting for pod %s to become RUNNING after %s", podID, timeout)
		case <-ticker.C:
			status, err := client.GetPodStatus(podID)
			if err != nil {
				fmt.Fprint(stderr, "e")
				continue
			}
			if status.Status == "RUNNING" {
				fmt.Fprintln(stderr)
				return nil
			}
			fmt.Fprint(stderr, ".")
		}
	}
}

// CheckModelStatus queries the vLLM API to check if a model is loaded and ready.
func CheckModelStatus(baseURL, modelName, apiKey string) (bool, error) {
	modelsURL := baseURL + "/models"

	req, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

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

	fmt.Fprintf(os.Stderr, "Models response: %s\n", string(respBody))

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

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
			if modelID == modelName {
				return true, nil
			}
			if modelID == modelName+":latest" {
				return true, nil
			}
		}
	}

	return false, nil
}

// PullOllamaModel pulls a model in Ollama via the /api/pull endpoint.
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

	maxRetries := 12
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPost, pullURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create pull request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Minute}
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
