package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

var availabilityJSON bool
var availabilityAllClouds bool
var availabilityRegion string
var availabilityCudaVersion string

// formatAvailability converts max GPU count to a human-readable availability string
func formatAvailability(maxCount int) string {
	if maxCount > 10 {
		return fmt.Sprintf("High (%d)", maxCount)
	}
	if maxCount > 0 {
		return fmt.Sprintf("Limited (%d)", maxCount)
	}
	return "Unavailable"
}

// orEmpty returns "(any)" if s is empty, otherwise returns s
func orEmpty(s string) string {
	if s == "" {
		return "(any)"
	}
	return s
}

var availabilityCmd = &cobra.Command{
	Use:   "availability",
	Short: "List available GPU types with pricing and specifications",
	RunE:  runAvailability,
}

func init() {
	availabilityCmd.Flags().BoolVar(&availabilityJSON, "json", false, "output result as JSON")
	availabilityCmd.Flags().BoolVar(&availabilityAllClouds, "all-clouds", false, "show both secure and community cloud availability (default: secure only)")
	availabilityCmd.Flags().StringVar(&availabilityRegion, "region", "", "filter by region (overrides config; empty = any region)")
	availabilityCmd.Flags().StringVar(&availabilityCudaVersion, "cuda-version", "", "filter by CUDA version (overrides config; empty = any CUDA version)")
}

func runAvailability(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Resolve effective region and CUDA version from flags or config
	region := availabilityRegion
	if region == "" {
		region = cfg.Region
	}
	cudaVersion := availabilityCudaVersion
	if cudaVersion == "" {
		cudaVersion = cfg.CudaVersion
	}

	client := newPodClient(cfg.RunpodAPIKey)
	gpuTypes, err := client.GetGPUTypes()
	if err != nil {
		return fmt.Errorf("failed to query GPU types: %w", err)
	}

	// Display filter info to user
	fmt.Fprintf(cmd.ErrOrStderr(), "Filters: Cloud=Secure, Region=%s, CudaVersion=%s\n\n",
		orEmpty(region), orEmpty(cudaVersion))

	// Always filter to secure cloud (matches CreatePod behavior)
	var filtered []pod.GPUType
	for _, gpu := range gpuTypes {
		if !gpu.SecureCloud {
			continue
		}
		// If GPU has zero availability in secure cloud, skip it
		if gpu.MaxGpuCountSecureCloud == 0 {
			continue
		}
		filtered = append(filtered, gpu)
	}
	gpuTypes = filtered

	// Sort by secure cloud price (cheapest first), then by ID for consistent ordering
	sort.Slice(gpuTypes, func(i, j int) bool {
		if gpuTypes[i].SecurePrice != gpuTypes[j].SecurePrice {
			return gpuTypes[i].SecurePrice < gpuTypes[j].SecurePrice
		}
		return gpuTypes[i].ID < gpuTypes[j].ID
	})

	if availabilityJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(gpuTypes)
	}

	// Print human-readable table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	fmt.Fprintf(cmd.OutOrStdout(), "Available GPUs (Secure Cloud, Region=%s, CUDA=%s):\n\n",
		orEmpty(region), orEmpty(cudaVersion))

	if availabilityAllClouds {
		fmt.Fprintln(w, "GPU TYPE ID\tNAME\tMEMORY\tSECURE AVAIL\tCOMMUNITY AVAIL\tSECURE PRICE\tCOMMUNITY PRICE")
		for _, gpu := range gpuTypes {
			secureAvail := formatAvailability(gpu.MaxGpuCountSecureCloud)
			communityAvail := formatAvailability(gpu.MaxGpuCountCommunityCloud)
			fmt.Fprintf(w, "%s\t%s\t%dGB\t%s\t%s\t$%.4f/hr\t$%.4f/hr\n",
				gpu.ID,
				gpu.DisplayName,
				gpu.MemoryInGb,
				secureAvail,
				communityAvail,
				gpu.SecurePrice,
				gpu.CommunityPrice,
			)
		}
	} else {
		fmt.Fprintln(w, "GPU TYPE ID\tNAME\tMEMORY\tAVAILABILITY\tSECURE PRICE")
		for _, gpu := range gpuTypes {
			avail := formatAvailability(gpu.MaxGpuCountSecureCloud)
			fmt.Fprintf(w, "%s\t%s\t%dGB\t%s\t$%.4f/hr\n",
				gpu.ID,
				gpu.DisplayName,
				gpu.MemoryInGb,
				avail,
				gpu.SecurePrice,
			)
		}
	}
	w.Flush()

	fmt.Fprintf(cmd.OutOrStdout(), "\nThese GPUs are deployable with your current constraints. Use with: runpod-launcher up\n")
	return nil
}
