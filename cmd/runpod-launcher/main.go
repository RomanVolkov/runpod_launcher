package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// newPodClient is a package-level factory for creating PodClient instances.
// Tests override this to inject mocks without touching real HTTP.
var newPodClient func(apiKey string) pod.PodClient = pod.NewRunPodClient

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "runpod-launcher",
	Short: "Spin up and tear down a RunPod GPU pod running an LLM via RunPod proxy",
	Long: `runpod-launcher manages a RunPod GPU pod running vLLM, exposed through a
RunPod direct proxy so your OpenCode config never needs updating.

Usage:
  runpod-launcher up                - create pod (interactive GPU selection available)
  runpod-launcher down              - terminate pod and stop billing
  runpod-launcher status            - check pod status
  runpod-launcher model-status      - check if a model is loaded and ready
  runpod-launcher availability      - list available GPU types with pricing
  runpod-launcher init              - create default config file

Interactive Features:
  The 'up' command will ask if you want to select a different GPU with a
  beautiful TUI that supports filtering, navigation, and sorting by price.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/runpod-launcher/config.toml)")
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(modelStatusCmd)
	rootCmd.AddCommand(availabilityCmd)
	rootCmd.AddCommand(initCmd)
}
