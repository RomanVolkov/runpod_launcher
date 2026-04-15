package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// selectGPUType presents the user with an interactive GPU selection using bubble tea TUI.
// It fetches available GPUs, filters to secure-only, and lets the user pick one.
// Returns the selected GPU's ID.
func selectGPUType(cmd *cobra.Command, client pod.PodClient, currentGPUTypeID string) (string, error) {
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stderr, "Fetching available GPUs...\n")

	gpuTypes, err := client.GetGPUTypes()
	if err != nil {
		return "", fmt.Errorf("failed to query GPU types: %w", err)
	}

	// Filter to secure-only GPUs with good stock status
	var secureGPUs []pod.GPUType
	for _, gpu := range gpuTypes {
		if gpu.SecureCloud {
			// Prefer GPUs with known good stock status (High, Medium)
			// Still include unknown/low stock, as they may become available
			secureGPUs = append(secureGPUs, gpu)
		}
	}

	if len(secureGPUs) == 0 {
		return "", fmt.Errorf("no GPUs available in secure cloud")
	}

	// Sort by stock status (High > Medium > Low > Unknown), then by price
	sortGPUsByStockAndPrice(secureGPUs)

	// Use bubble tea TUI for selection
	selected, err := selectGPUTypeTUI(secureGPUs)
	if err != nil {
		return "", err
	}

	return selected, nil
}

// availabilityRank returns a rank for sorting based on max GPU count (higher is better availability)
func availabilityRank(maxCount int) int {
	if maxCount > 10 {
		return 3 // High availability
	}
	if maxCount > 0 {
		return 2 // Limited availability
	}
	return 0 // Unavailable
}

// sortGPUsByAvailabilityAndPrice sorts GPUs by availability (best first), then by price (cheapest first)
func sortGPUsByStockAndPrice(gpus []pod.GPUType) {
	for i := 0; i < len(gpus); i++ {
		for j := i + 1; j < len(gpus); j++ {
			iRank := availabilityRank(gpus[i].MaxGpuCountSecureCloud)
			jRank := availabilityRank(gpus[j].MaxGpuCountSecureCloud)

			// First, sort by availability (descending)
			if jRank != iRank {
				if jRank > iRank {
					gpus[i], gpus[j] = gpus[j], gpus[i]
				}
			} else {
				// If same availability, sort by price (ascending)
				if gpus[j].SecurePrice < gpus[i].SecurePrice {
					gpus[i], gpus[j] = gpus[j], gpus[i]
				}
			}
		}
	}
}

// isTerminal checks if the given file descriptor is a terminal
// Returns true only if we can verify it's connected to a terminal
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	// In Unix-like systems, terminals are character devices with mode flags 0o20000
	// We check if it's not a regular file and not a pipe
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// promptForGPUSelection asks the user if they want to select a different GPU.
// If upSelectGPU flag is set, skips the confirmation and goes directly to selection.
// Skips interactive prompts in JSON mode or when stdin is not a terminal.
func promptForGPUSelection(cmd *cobra.Command, client pod.PodClient, cfg *config.Config) error {
	stderr := cmd.ErrOrStderr()

	// Skip interactive prompts in JSON mode or when forced to select
	var shouldSelectGPU bool

	if upSelectGPU {
		// Force GPU selection
		shouldSelectGPU = true
	} else if upJSON {
		// Skip interactive prompts in JSON mode
		return nil
	} else if !isTerminal(os.Stdin) {
		// Skip interactive prompts when stdin is not a terminal (tests, pipes, etc)
		return nil
	} else {
		// Ask user interactively
		fmt.Fprintf(stderr, "\nCurrent GPU: %s\n", cfg.GPUTypeID)
		fmt.Fprintf(stderr, "Do you want to select a different GPU? (y/n) [n]: ")

		var input string
		_, err := fmt.Scanln(&input)
		if err != nil || input == "" || (input != "y" && input != "Y" && input != "yes" && input != "YES") {
			return nil // Don't select, use current GPU
		}
		shouldSelectGPU = true
	}

	if !shouldSelectGPU {
		return nil
	}

	// User wants to select a GPU
	selectedGPU, err := selectGPUType(cmd, client, cfg.GPUTypeID)
	if err != nil {
		return fmt.Errorf("failed to select GPU: %w", err)
	}

	cfg.GPUTypeID = selectedGPU
	fmt.Fprintf(stderr, "Selected GPU: %s\n", selectedGPU)

	return nil
}
