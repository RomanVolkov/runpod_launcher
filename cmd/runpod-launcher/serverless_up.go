package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/opencode"
	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

var serverlessUpJSON bool

var newServerlessClient func(apiKey string) serverless.Client = serverless.NewRunPodServerlessClient

var serverlessUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Create or activate a RunPod serverless endpoint",
	RunE:  runServerlessUp,
}

func init() {
	serverlessUpCmd.Flags().BoolVar(&serverlessUpJSON, "json", false, "output result as JSON")
}

func runServerlessUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	endpointName := cfg.ServerlessEndpointName
	if endpointName == "" {
		endpointName = serverless.DefaultEndpointName
	}
	imageName := cfg.ServerlessImageName
	if imageName == "" {
		imageName = serverless.DefaultImageName
	}
	workersMax := cfg.ServerlessWorkersMax
	if workersMax == 0 {
		workersMax = serverless.DefaultWorkersMax
	}

	// Determine model name: use ServerlessModelName if set, fallback to ModelName
	modelName := cfg.ServerlessModelName
	if modelName == "" {
		modelName = cfg.ModelName
	}

	client := newServerlessClient(cfg.RunpodAPIKey)

	// Check if endpoint already exists
	existing, err := client.FindEndpointByName(endpointName)
	if err != nil {
		return fmt.Errorf("failed to check for existing endpoint: %w", err)
	}
	if existing != nil {
		return printServerlessUpResult(cmd, serverlessUpJSON, existing.ID, true, cfg)
	}

	// Create template with vLLM options
	diskGB := cfg.ServerlessContainerDiskGB
	if diskGB == 0 {
		diskGB = 50
	}
	// Build vLLM template options from config
	opts := &serverless.VLLMTemplateOptions{
		MaxModelLen:   cfg.ServerlessMaxModelLen,
		Dtype:         cfg.ServerlessDtype,
		GpuMemoryUtil: cfg.ServerlessGpuMemoryUtilization,
	}
	templateID, err := client.CreateTemplate(endpointName, imageName, modelName, diskGB, opts)
	if err != nil {
		return fmt.Errorf("failed to create serverless template: %w", err)
	}

	// Create endpoint
	gpuTypeID := cfg.ServerlessGPUTypeID
	if gpuTypeID == "" {
		gpuTypeID = "NVIDIA A100-SXM4-80GB"
	}
	endpointID, err := client.CreateEndpoint(
		endpointName,
		gpuTypeID,
		templateID,
		workersMax,
		serverless.DefaultIdleTimeout,
		serverless.DefaultScalerValue,
		serverless.DefaultScalerType,
	)
	if err != nil {
		return fmt.Errorf("failed to create serverless endpoint: %w", err)
	}

	return printServerlessUpResult(cmd, serverlessUpJSON, endpointID, false, cfg)
}

func serverlessBaseURL(endpointID string) string {
	return fmt.Sprintf("https://api.runpod.ai/v2/%s/openai/v1", endpointID)
}

func printServerlessUpResult(cmd *cobra.Command, asJSON bool, endpointID string, alreadyExists bool, cfg *config.Config) error {
	url := serverlessBaseURL(endpointID)

	openCodeUpdated := false
	envFileUpdated := false

	// Get the OpenCode config path
	openCodePath := cfg.OpenCodeConfigPath

	// Update OpenCode config with env var reference and write API key to env file
	if openCodePath != "" {
		modelName := cfg.ServerlessModelName
		if modelName == "" {
			modelName = cfg.ModelName
		}

		// Get env file path (default to ~/.env)
		envFilePath := cfg.EnvFilePath
		if envFilePath == "" {
			envFilePath = "~/.env"
		}

		// Write API key to env file
		if err := opencode.WriteEnvVarToFile("RUNPOD_API_KEY", cfg.RunpodAPIKey, envFilePath); err == nil {
			envFileUpdated = true
		}

		// Update OpenCode config with env var reference (not the real key)
		if err := opencode.UpdateConfigWithProviderAndEnv(openCodePath, url, modelName, "runpod-serverless", "RUNPOD_API_KEY"); err == nil {
			openCodeUpdated = true
		}
	}

	if asJSON {
		out := map[string]interface{}{
			"status":           serverless.StatusActive,
			"endpoint_id":      endpointID,
			"url":              url,
			"opencode_updated": openCodeUpdated,
			"env_updated":      envFileUpdated,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	if alreadyExists {
		fmt.Fprintf(cmd.OutOrStdout(), "Serverless endpoint already exists: %s\nURL: %s\n", endpointID, url)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Serverless endpoint created: %s\nURL: %s\n", endpointID, url)
	}

	if openCodeUpdated && envFileUpdated {
		fmt.Fprintf(cmd.OutOrStdout(), "✓ OpenCode config updated (URL: %s)\n", openCodePath)
		fmt.Fprintf(cmd.OutOrStdout(), "✓ API key written to ~/.env as RUNPOD_API_KEY\n")
	} else if !openCodeUpdated || !envFileUpdated {
		fmt.Fprintf(cmd.OutOrStdout(), "Configure OpenCode with:\n  URL: %s\n  API Key: {env:RUNPOD_API_KEY}\n  Model: %s\n", url, "runpod-serverless")
		if !envFileUpdated {
			fmt.Fprintf(cmd.OutOrStdout(), "Add to ~/.env: RUNPOD_API_KEY=%s\n", cfg.RunpodAPIKey)
		}
	}

	return nil
}
