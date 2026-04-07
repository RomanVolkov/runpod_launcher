package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether the RunPod pod is running",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output result as JSON")
}

func runStatus(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("failed to check pod status: %w", err)
	}

	// When a pod is found, fetch its actual desiredStatus for accurate reporting.
	var desiredStatus string
	if podID != "" {
		ps, err := client.GetPodStatus(podID)
		if err != nil {
			return fmt.Errorf("failed to get pod status: %w", err)
		}
		desiredStatus = ps.DesiredStatus
	}

	return printStatusResult(cmd, statusJSON, podID, desiredStatus)
}

func printStatusResult(cmd *cobra.Command, asJSON bool, podID, desiredStatus string) error {
	if asJSON {
		type statusOutput struct {
			Status string  `json:"status"`
			PodID  *string `json:"pod_id"`
		}
		out := statusOutput{}
		if podID != "" {
			out.Status = desiredStatus
			out.PodID = &podID
		} else {
			out.Status = pod.StatusNotFound
			out.PodID = nil
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	if podID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Pod status: %s (%s)\n", desiredStatus, podID)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Pod not found")
	}
	return nil
}
