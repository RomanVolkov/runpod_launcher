package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

var serverlessDownJSON bool

var serverlessDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Scale a RunPod serverless endpoint to zero workers",
	RunE:  runServerlessDown,
}

func init() {
	serverlessDownCmd.Flags().BoolVar(&serverlessDownJSON, "json", false, "output result as JSON")
}

func runServerlessDown(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	endpointName := cfg.ServerlessEndpointName
	if endpointName == "" {
		endpointName = serverless.DefaultEndpointName
	}

	client := newServerlessClient(cfg.RunpodAPIKey)

	existing, err := client.FindEndpointByName(endpointName)
	if err != nil {
		return fmt.Errorf("failed to look up endpoint: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("serverless endpoint %q not found", endpointName)
	}

	// Scale to zero: workersMin=0, workersMax=0
	_, err = client.SaveEndpoint(
		existing.ID,
		existing.Name,
		"",  // gpuIDs: empty string — API keeps existing GPU config when updating
		"",  // templateID: empty string — API keeps existing template when updating
		0,   // workersMin
		0,   // workersMax
		serverless.DefaultIdleTimeout,
		serverless.DefaultScalerValue,
		serverless.DefaultScalerType,
	)
	if err != nil {
		return fmt.Errorf("failed to scale endpoint to zero: %w", err)
	}

	return printServerlessDownResult(cmd, serverlessDownJSON, existing.ID)
}

func printServerlessDownResult(cmd *cobra.Command, asJSON bool, endpointID string) error {
	if asJSON {
		out := map[string]string{
			"status":      serverless.StatusScaledToZero,
			"endpoint_id": endpointID,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Serverless endpoint scaled to zero: %s\n", endpointID)
	return nil
}
