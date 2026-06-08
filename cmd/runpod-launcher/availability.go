package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/graphql"
	"github.com/romanvolkov/runpod-launcher/internal/models"
)

var availabilityJSON bool
var availabilityModel string

// GPUWithPrice combines GPU info with pricing
type GPUWithPrice struct {
	ID               string  `json:"id"`
	DisplayName      string  `json:"displayName"`
	MemoryGb         int     `json:"memoryGb"`
	Price            float64 `json:"price,omitempty"`
	Suitability      string  `json:"suitability,omitempty"`
	SuitabilityScore float64 `json:"suitabilityScore,omitempty"`
}

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
	Short: "List available GPUs with pricing (via GraphQL API)",
	RunE:  runAvailability,
}

func init() {
	availabilityCmd.Flags().BoolVar(&availabilityJSON, "json", false, "output as JSON")
	availabilityCmd.Flags().StringVar(&availabilityModel, "model", "", "filter/highlight GPUs suitable for model (e.g., 'qwen3.6:27b')")
}

func runAvailability(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}

	// Create GraphQL client
	gqlClient, err := graphql.NewClient(cfg.RunpodAPIKey)
	if err != nil {
		return fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	// Fetch GPU types
	gpuTypes, err := gqlClient.GetGPUTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch GPU types: %w", err)
	}

	if len(gpuTypes) == 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "No GPUs available\n")
		return nil
	}

	// Fetch pricing info
	priceInput := &graphql.GPULowestPriceInput{
		GpuCount:    1,
		SecureCloud: boolPtr(true),
	}
	prices, err := gqlClient.GetLowestPrice(priceInput)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not fetch pricing: %v\n", err)
		prices = nil
	}

	// Build price map for quick lookup
	priceMap := make(map[string]float64)
	for _, p := range prices {
		if p.UninterruptablePrice > 0 {
			priceMap[p.GPUTypeID] = p.UninterruptablePrice
		} else {
			priceMap[p.GPUTypeID] = p.MinimumBidPrice
		}
	}

	// Filter to secure cloud only and convert to simple structure
	var gpuList []GPUWithPrice
	for _, gpu := range gpuTypes {
		if !gpu.SecureCloud {
			continue
		}

		price := priceMap[gpu.ID]
		item := GPUWithPrice{
			ID:          gpu.ID,
			DisplayName: gpu.DisplayName,
			MemoryGb:    gpu.MemoryInGb,
			Price:       price,
		}

		// If model specified, calculate suitability
		if availabilityModel != "" {
			modelSpec, err := models.GetModelSpec(availabilityModel, cfg.ModelSpecsOverride)
			if err != nil {
				return fmt.Errorf("unknown model %q: %w", availabilityModel, err)
			}

			gpuInfo := models.GPUInfo{
				ID:       gpu.ID,
				Name:     gpu.DisplayName,
				MemoryGb: gpu.MemoryInGb,
			}
			suitability := models.CalculateSuitability(gpuInfo, modelSpec)
			item.Suitability = suitability.Level
			item.SuitabilityScore = suitability.SuitabilityScore
		}

		gpuList = append(gpuList, item)
	}

	// Sort by price (cheapest first), then by ID
	sort.Slice(gpuList, func(i, j int) bool {
		if gpuList[i].Price != gpuList[j].Price {
			return gpuList[i].Price < gpuList[j].Price
		}
		return gpuList[i].ID < gpuList[j].ID
	})

	if availabilityJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(gpuList)
	}

	// Print human-readable table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(cmd.OutOrStdout(), "Available GPUs (Secure Cloud):\n\n")

	if availabilityModel != "" {
		fmt.Fprintln(w, "GPU ID\tNAME\tMEMORY\tSUITABILITY\tPRICE")
		for _, gpu := range gpuList {
			suitLabel := gpu.Suitability
			if suitLabel == "" {
				suitLabel = "?"
			}
			priceStr := "N/A"
			if gpu.Price > 0 {
				priceStr = fmt.Sprintf("$%.4f/hr", gpu.Price)
			}
			fmt.Fprintf(w, "%s\t%s\t%dGB\t%s\t%s\n",
				gpu.ID, gpu.DisplayName, gpu.MemoryGb, suitLabel, priceStr)
		}
	} else {
		fmt.Fprintln(w, "GPU ID\tNAME\tMEMORY\tPRICE")
		for _, gpu := range gpuList {
			priceStr := "N/A"
			if gpu.Price > 0 {
				priceStr = fmt.Sprintf("$%.4f/hr", gpu.Price)
			}
			fmt.Fprintf(w, "%s\t%s\t%dGB\t%s\n",
				gpu.ID, gpu.DisplayName, gpu.MemoryGb, priceStr)
		}
	}
	w.Flush()

	fmt.Fprintf(cmd.OutOrStdout(), "\nNote: Prices from GraphQL API (real-time).\n")
	if availabilityModel != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Suitability for model %q: green=suitable, yellow=marginal, red=insufficient\n", availabilityModel)
	}
	return nil
}

func boolPtr(b bool) *bool {
	return &b
}
