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

var availabilityCmd = &cobra.Command{
	Use:   "availability",
	Short: "List available GPU types with pricing and specifications",
	RunE:  runAvailability,
}

func init() {
	availabilityCmd.Flags().BoolVar(&availabilityJSON, "json", false, "output result as JSON")
	availabilityCmd.Flags().BoolVar(&availabilityAllClouds, "all-clouds", false, "show both secure and community cloud availability (default: secure only)")
}

func runAvailability(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	client := newPodClient(cfg.RunpodAPIKey)
	gpuTypes, err := client.GetGPUTypes()
	if err != nil {
		return fmt.Errorf("failed to query GPU types: %w", err)
	}

	// Filter to secure availability by default
	if !availabilityAllClouds {
		var filtered []pod.GPUType
		for _, gpu := range gpuTypes {
			if gpu.SecureCloud {
				filtered = append(filtered, gpu)
			}
		}
		gpuTypes = filtered
	}

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

	return nil
}
