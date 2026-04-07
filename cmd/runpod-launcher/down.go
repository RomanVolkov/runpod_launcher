package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

var downJSON bool

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Terminate the running RunPod pod",
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolVar(&downJSON, "json", false, "output result as JSON")
}

func runDown(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	client := newPodClient(cfg.RunpodAPIKey)

	podName := cfg.PodName
	if podName == "" {
		podName = pod.DefaultPodName
	}

	podID, err := client.FindPodByName(podName)
	if err != nil {
		return fmt.Errorf("failed to look up pod: %w", err)
	}
	if podID == "" {
		return fmt.Errorf("pod %q not found", podName)
	}

	if err := client.TerminatePod(podID); err != nil {
		return fmt.Errorf("failed to terminate pod: %w", err)
	}

	return printDownResult(cmd, downJSON, podID)
}

func printDownResult(cmd *cobra.Command, asJSON bool, podID string) error {
	if asJSON {
		out := map[string]string{
			"status": pod.StatusTerminated,
			"pod_id": podID,
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Pod terminated: %s\n", podID)
	return nil
}
