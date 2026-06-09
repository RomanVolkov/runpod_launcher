package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/models"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// selectGPUType presents the user with an interactive GPU selection using bubble tea TUI.
// It fetches available GPUs, filters to secure-only, and lets the user pick one.
// If modelName is provided, pre-filters GPUs suitable for that model.
// Returns the selected GPU's ID.
func selectGPUType(cmd *cobra.Command, client pod.PodClient, currentGPUTypeID, region, cudaVersion, modelName string, modelOverrides map[string]models.ModelSpec) (string, error) {
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

	// If model specified, pre-filter GPUs by minimum VRAM requirements
	if modelName != "" {
		modelSpec, err := models.GetModelSpec(modelName, modelOverrides)
		if err != nil {
			fmt.Fprintf(stderr, "Warning: could not get model specs for %q: %v\n", modelName, err)
		} else {
			var filtered []pod.GPUType
			for _, gpu := range secureGPUs {
				if gpu.MemoryInGb >= modelSpec.MinVramGb {
					filtered = append(filtered, gpu)
				}
			}
			if len(filtered) > 0 {
				secureGPUs = filtered
				fmt.Fprintf(stderr, "Pre-filtered GPUs suitable for %s (min %dGB)\n", modelName, modelSpec.MinVramGb)
			}
		}
	}

	// Sort by stock status (High > Medium > Low > Unknown), then by price
	sortGPUsByStockAndPrice(secureGPUs)

	// Use bubble tea TUI for selection
	selected, err := selectGPUTypeTUI(secureGPUs, region, cudaVersion)
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

// promptForModelSelection allows user to interactively select a model.
// Always prompts for model selection, even if one is already configured.
// Returns true if model was selected/confirmed interactively, false otherwise.
func promptForModelSelection(cmd *cobra.Command, cfg *config.Config) (bool, error) {
	stderr := cmd.ErrOrStderr()

	// Skip interactive prompts in JSON mode or when stdin is not a terminal
	if upJSON || !isTerminal(os.Stdin) {
		return false, nil
	}

	// Get available models
	availableModels := models.ListAvailableModels(cfg.ModelSpecsOverride)
	if len(availableModels) == 0 {
		return false, nil
	}

	// If a model is already configured, ask if user wants to change it
	if cfg.ModelName != "" {
		fmt.Fprintf(stderr, "\nCurrent model: %s\n", cfg.ModelName)
		fmt.Fprintf(stderr, "Select a different model? (y/n) [n]: ")

		var input string
		_, err := fmt.Scanln(&input)
		if err != nil || input == "" || (input != "y" && input != "Y" && input != "yes" && input != "YES") {
			// Keep current model
			return false, nil
		}
		// User wants to select a different model, fall through to selection
	}

	// Present model selection
	fmt.Fprintf(stderr, "\nAvailable models:\n")
	for i, m := range availableModels {
		fmt.Fprintf(stderr, "  %d. %s\n", i+1, m)
	}

	// Find index of current model to use as default
	defaultChoice := 1
	if cfg.ModelName != "" {
		for i, m := range availableModels {
			if m == cfg.ModelName {
				defaultChoice = i + 1
				break
			}
		}
	}

	fmt.Fprintf(stderr, "\nSelect a model (1-%d) [%d]: ", len(availableModels), defaultChoice)
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil || input == "" {
		input = fmt.Sprintf("%d", defaultChoice)
	}

	var choice int
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(availableModels) {
		choice = defaultChoice
	}

	cfg.ModelName = availableModels[choice-1]
	fmt.Fprintf(stderr, "Selected model: %s\n", cfg.ModelName)
	return true, nil
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
	selectedGPU, err := selectGPUType(cmd, client, cfg.GPUTypeID, cfg.Region, cfg.CudaVersion, cfg.ModelName, cfg.ModelSpecsOverride)
	if err != nil {
		return fmt.Errorf("failed to select GPU: %w", err)
	}

	cfg.GPUTypeID = selectedGPU
	fmt.Fprintf(stderr, "Selected GPU: %s\n", selectedGPU)

	return nil
}
