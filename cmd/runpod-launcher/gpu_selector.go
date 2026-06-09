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

// promptForModelSelection allows user to interactively select a model using TUI.
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
		// User wants to select a different model, fall through to TUI selection
	}

	// Build a map of model specs for display
	modelSpecsMap := make(map[string]models.ModelSpec)
	for _, modelName := range availableModels {
		spec, _ := models.GetModelSpec(modelName, cfg.ModelSpecsOverride)
		modelSpecsMap[modelName] = spec
	}

	// Use interactive TUI for model selection
	fmt.Fprintf(stderr, "\n")
	selectedModel, err := selectModelTUI(availableModels, modelSpecsMap)
	if err != nil {
		return false, err
	}

	cfg.ModelName = selectedModel
	fmt.Fprintf(stderr, "\nSelected model: %s\n", cfg.ModelName)
	return true, nil
}

// promptForGPUSelection allows user to interactively select a GPU.
// Always offers GPU selection after model is chosen, with pre-filtering based on model.
// Skips interactive prompts in JSON mode or when stdin is not a terminal.
func promptForGPUSelection(cmd *cobra.Command, client pod.PodClient, cfg *config.Config) error {
	stderr := cmd.ErrOrStderr()

	// Skip interactive prompts in JSON mode or when stdin is not a terminal
	if upJSON || !isTerminal(os.Stdin) {
		return nil
	}

	// Get model spec for pre-filtering and display
	modelSpec, _ := models.GetModelSpec(cfg.ModelName, cfg.ModelSpecsOverride)

	// Always offer GPU selection with model context
	fmt.Fprintf(stderr, "\nSelecting GPU for %s (%dGB VRAM min, %d token context):\n",
		cfg.ModelName, modelSpec.MinVramGb, modelSpec.ContextWindow)
	fmt.Fprintf(stderr, "Current GPU: %s\n", cfg.GPUTypeID)
	fmt.Fprintf(stderr, "Select a different GPU? (y/n) [n]: ")

	var input string
	_, err := fmt.Scanln(&input)
	if err != nil || input == "" || (input != "y" && input != "Y" && input != "yes" && input != "YES") {
		if upSelectGPU {
			// Force selection mode flag still applies
			fmt.Fprintf(stderr, "Using: %s\n", cfg.GPUTypeID)
		}
		return nil // Keep current GPU
	}

	// User wants to select a GPU - show options with model pre-filtering
	selectedGPU, err := selectGPUType(cmd, client, cfg.GPUTypeID, cfg.Region, cfg.CudaVersion, cfg.ModelName, cfg.ModelSpecsOverride)
	if err != nil {
		return fmt.Errorf("failed to select GPU: %w", err)
	}

	cfg.GPUTypeID = selectedGPU
	fmt.Fprintf(stderr, "Selected GPU: %s\n", selectedGPU)

	return nil
}
