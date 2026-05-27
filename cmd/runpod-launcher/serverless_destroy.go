package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

var serverlessDestroyJSON bool

var serverlessDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Permanently delete a RunPod serverless endpoint",
	RunE:  runServerlessDestroy,
}

func init() {
	serverlessDestroyCmd.Flags().BoolVar(&serverlessDestroyJSON, "json", false, "output result as JSON")
}

func runServerlessDestroy(cmd *cobra.Command, args []string) error {
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

	if err := client.DeleteEndpoint(existing.ID); err != nil {
		return fmt.Errorf("failed to delete endpoint: %w", err)
	}

	if serverlessDestroyJSON {
		out := map[string]string{"status": "deleted", "endpoint_id": existing.ID}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Serverless endpoint deleted: %s\n", existing.ID)
	return nil
}
