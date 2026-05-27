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
var serverlessUpOpenCodeConfig string

var newServerlessClient func(apiKey string) serverless.Client = serverless.NewRunPodServerlessClient
var updateServerlessOpenCodeConfig = opencode.UpdateConfigWithProvider

var serverlessUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Create or activate a RunPod serverless endpoint",
	RunE:  runServerlessUp,
}

func init() {
	serverlessUpCmd.Flags().BoolVar(&serverlessUpJSON, "json", false, "output result as JSON")
	serverlessUpCmd.Flags().StringVar(&serverlessUpOpenCodeConfig, "opencode-config", "",
		"path to OpenCode config JSON (optional; overrides config file value)")
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

	client := newServerlessClient(cfg.RunpodAPIKey)

	// Check if endpoint already exists
	existing, err := client.FindEndpointByName(endpointName)
	if err != nil {
		return fmt.Errorf("failed to check for existing endpoint: %w", err)
	}
	if existing != nil {
		return printServerlessUpResult(cmd, serverlessUpJSON, existing.ID, true, cfg)
	}

	// Create template
	diskGB := cfg.ContainerDiskGB
	if diskGB == 0 {
		diskGB = 50
	}
	templateID, err := client.SaveTemplate(endpointName, imageName, cfg.ModelName, cfg.RunpodAPIKey, diskGB)
	if err != nil {
		return fmt.Errorf("failed to create serverless template: %w", err)
	}

	// Create endpoint
	endpointID, err := client.SaveEndpoint(
		"",          // endpointID (empty for new)
		endpointName,
		cfg.GPUTypeID,
		templateID,
		0,                              // workersMin
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

	openCodePath := serverlessUpOpenCodeConfig
	if openCodePath == "" {
		openCodePath = cfg.OpenCodeConfigPath
	}

	openCodeUpdated := false
	if openCodePath != "" {
		if err := updateServerlessOpenCodeConfig(openCodePath, url, cfg.RunpodAPIKey, cfg.ModelName, "runpod-serverless"); err != nil {
			return fmt.Errorf("failed to update OpenCode config: %w", err)
		}
		openCodeUpdated = true
	}

	if asJSON {
		out := map[string]interface{}{
			"status":           serverless.StatusActive,
			"endpoint_id":      endpointID,
			"url":              url,
			"opencode_updated": openCodeUpdated,
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
	if openCodeUpdated {
		fmt.Fprintf(cmd.OutOrStdout(), "OpenCode config updated: %s\n", openCodePath)
	}
	return nil
}
