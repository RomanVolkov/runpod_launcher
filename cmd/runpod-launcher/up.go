package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/opencode"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
	"github.com/romanvolkov/runpod-launcher/internal/util"
)

var upJSON bool
var upOpenCodeConfig string
var upRegion string

// upWaitTimeout is the maximum time to wait for a pod to become RUNNING.
// Tests override this to keep test execution fast.
var upWaitTimeout = 5 * time.Minute

// upWaitTick is the polling interval passed to WaitForReady.
// Tests override this to avoid real-time delays.
var upWaitTick = 5 * time.Second

// updateOpenCodeConfig is injected for testing to allow mocking UpdateConfig.
var updateOpenCodeConfig = opencode.UpdateConfig

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Create a RunPod pod and wait until it is ready",
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVar(&upJSON, "json", false, "output result as JSON")
	upCmd.Flags().StringVar(&upOpenCodeConfig, "opencode-config", "", "path to OpenCode config JSON (optional; overrides config file value)")
	upCmd.Flags().StringVar(&upRegion, "region", "", "RunPod region (optional; overrides config file value; e.g. EU, US-EAST)")
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Override region from flag if provided
	if upRegion != "" {
		cfg.Region = upRegion
	}

	// Generate a strong random API key for this pod
	llmAPIKey, err := util.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	client := newPodClient(cfg.RunpodAPIKey)

	podName := cfg.PodName
	if podName == "" {
		podName = pod.DefaultPodName
	}

	// Check for an existing pod by name; if already running, print existing info and exit 0.
	existingID, err := client.FindPodByName(podName)
	if err != nil {
		return fmt.Errorf("failed to check for existing pod: %w", err)
	}
	if existingID != "" {
		return printUpResult(cmd, upJSON, existingID, true, cfg, "")
	}

	podID, err := client.CreatePod(cfg, llmAPIKey)
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	if err := pod.WaitForReady(client, podID, upWaitTimeout, cmd.ErrOrStderr(), upWaitTick); err != nil {
		return err
	}

	// Handle model loading based on image type
	baseURL := fmt.Sprintf("https://%s-8000.proxy.runpod.net", podID)

	// If using Ollama, pull the model first
	if strings.Contains(cfg.ImageName, "ollama") {
		fmt.Fprintf(cmd.ErrOrStderr(), "Pulling Ollama model\n")
		if err := pod.PullOllamaModel(baseURL, cfg.ModelName, cmd.ErrOrStderr()); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
		}
	}

	// Wait for the model to be loaded
	fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for model to load")
	vllmURL := baseURL + "/v1"
	if err := pod.WaitForModelReady(vllmURL, cfg.ModelName, llmAPIKey, upWaitTimeout, cmd.ErrOrStderr(), upWaitTick); err != nil {
		// Log but don't fail - the pod is ready, model just taking longer to load
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
	}

	// Save the generated API key to config for future use
	if err := cfg.SaveAPIKey(llmAPIKey); err != nil {
		// Log but don't fail - the pod is already created and ready
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to save API key to config: %v\n", err)
	}

	return printUpResult(cmd, upJSON, podID, false, cfg, llmAPIKey)
}

// podProxyURL returns the RunPod proxy URL for the given pod ID and port.
func podProxyURL(podID string, port int) string {
	return fmt.Sprintf("https://%s-%d.proxy.runpod.net", podID, port)
}

func printUpResult(cmd *cobra.Command, asJSON bool, podID string, alreadyRunning bool, cfg *config.Config, llmAPIKey string) error {
	url := podProxyURL(podID, pod.DefaultServicePort)

	// Resolve effective OpenCode config path: flag takes precedence over config file
	openCodePath := upOpenCodeConfig
	if openCodePath == "" {
		openCodePath = cfg.OpenCodeConfigPath
	}

	// Call OpenCode updater if path is non-empty and we have an API key
	// (API key is only provided for newly created pods, not for existing pods)
	openCodeUpdated := false
	if openCodePath != "" && llmAPIKey != "" {
		if err := updateOpenCodeConfig(openCodePath, url+"/v1", llmAPIKey, cfg.ModelName); err != nil {
			return fmt.Errorf("failed to update OpenCode config: %w", err)
		}
		openCodeUpdated = true
	}

	if asJSON {
		out := map[string]interface{}{
			"status":           pod.StatusRunning,
			"pod_id":           podID,
			"url":              url,
			"api_key":          llmAPIKey,
			"opencode_updated": openCodeUpdated,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	if alreadyRunning {
		fmt.Fprintf(cmd.OutOrStdout(), "Pod already running: %s\nURL: %s\n", podID, url)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Pod is ready: %s\nURL: %s\n", podID, url)
		if llmAPIKey != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "API Key: %s\n", llmAPIKey)
		}
	}
	if openCodeUpdated {
		fmt.Fprintf(cmd.OutOrStdout(), "OpenCode config updated: %s\n", openCodePath)
	}
	return nil
}
