package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

var modelStatusJSON bool
var modelStatusAPIKey string

var modelStatusCmd = &cobra.Command{
	Use:   "model-status [model-name]",
	Short: "Check if a model is loaded and ready on the running pod",
	Long: `Check if a model is loaded and ready on the running pod.

If model-name is not provided, it reads from the config file (model_name field).
If --api-key is not provided, the server is assumed to have no authentication.`,
	RunE: runModelStatus,
}

func init() {
	modelStatusCmd.Flags().BoolVar(&modelStatusJSON, "json", false, "output result as JSON")
	modelStatusCmd.Flags().StringVar(&modelStatusAPIKey, "api-key", "", "vLLM API key (if server requires authentication)")
}

func runModelStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Get model name from args or config
	modelName := cfg.ModelName
	if len(args) > 0 {
		modelName = args[0]
	}

	if modelName == "" {
		return fmt.Errorf("model name not provided and not set in config")
	}

	// Get API key from flag or config
	apiKey := modelStatusAPIKey
	if apiKey == "" {
		apiKey = cfg.LastLLMAPIKey
	}

	// Find the running pod
	client := newPodClient(cfg.RunpodAPIKey)

	podName := cfg.PodName
	if podName == "" {
		podName = pod.DefaultPodName
	}

	podID, err := client.FindPodByName(podName)
	if err != nil {
		return fmt.Errorf("failed to find pod: %w", err)
	}

	if podID == "" {
		return fmt.Errorf("pod not found")
	}

	// Get pod status to get the proxy URL
	podStatus, err := client.GetPodStatus(podID)
	if err != nil {
		return fmt.Errorf("failed to get pod status: %w", err)
	}

	// Construct the vLLM API base URL
	// Format: https://{pod-id}-8000.proxy.runpod.net/v1
	vllmURL := fmt.Sprintf("https://%s-8000.proxy.runpod.net/v1", podID)

	// Check if the model is loaded
	isLoaded, err := pod.CheckModelStatus(vllmURL, modelName, apiKey)
	if err != nil {
		return fmt.Errorf("failed to check model status: %w", err)
	}

	return printModelStatusResult(cmd, modelStatusJSON, modelName, isLoaded, podStatus.DesiredStatus)
}

func printModelStatusResult(cmd *cobra.Command, asJSON bool, modelName string, isLoaded bool, podStatus string) error {
	if asJSON {
		type modelStatusOutput struct {
			Model  string `json:"model"`
			Loaded bool   `json:"loaded"`
			Status string `json:"status"`
		}
		out := modelStatusOutput{
			Model:  modelName,
			Loaded: isLoaded,
			Status: podStatus,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	status := "not loaded"
	if isLoaded {
		status = "loaded"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Model %q is %s (pod status: %s)\n", modelName, status, podStatus)
	return nil
}
