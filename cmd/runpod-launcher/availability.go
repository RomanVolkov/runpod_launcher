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
			communityAvail := "No"
			if gpu.CommunityCloud {
				communityAvail = "Yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%dGB\tYes\t%s\t$%.4f/hr\t$%.4f/hr\n",
				gpu.ID,
				gpu.DisplayName,
				gpu.MemoryInGb,
				communityAvail,
				gpu.SecurePrice,
				gpu.CommunityPrice,
			)
		}
	} else {
		fmt.Fprintln(w, "GPU TYPE ID\tNAME\tMEMORY\tSECURE PRICE")
		for _, gpu := range gpuTypes {
			fmt.Fprintf(w, "%s\t%s\t%dGB\t$%.4f/hr\n",
				gpu.ID,
				gpu.DisplayName,
				gpu.MemoryInGb,
				gpu.SecurePrice,
			)
		}
	}
	w.Flush()

	return nil
}
