package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

func TestAvailability_JSONOutput(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		getGPUTypesFn: func() ([]pod.GPUType, error) {
			return []pod.GPUType{
				{
					ID:                        "AMPERE_16",
					DisplayName:               "RTX A4000",
					MemoryInGb:                16,
					SecurePrice:               0.44,
					CommunityPrice:            0.22,
					SecureSpotPrice:           0.22,
					CommunitySpotPrice:        0.11,
					SecureCloud:               true,
					CommunityCloud:            true,
					MaxGpuCountSecureCloud:    15,
					MaxGpuCountCommunityCloud: 8,
				},
				{
					ID:                        "ADA_LOVELACE_24",
					DisplayName:               "RTX 5880 Ada",
					MemoryInGb:                24,
					SecurePrice:               0.98,
					CommunityPrice:            0.49,
					SecureSpotPrice:           0.49,
					CommunitySpotPrice:        0.25,
					SecureCloud:               false,
					CommunityCloud:            true,
					MaxGpuCountCommunityCloud: 12,
				},
			}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	origCfgFile := cfgFile
	t.Cleanup(func() { cfgFile = origCfgFile })
	cfgFile = configPath

	var stdout bytes.Buffer
	availabilityCmd.SetOut(&stdout)
	availabilityCmd.SetErr(bytes.NewBuffer(nil))

	// Test with JSON flag
	origAvailabilityJSON := availabilityJSON
	origAvailabilityAllClouds := availabilityAllClouds
	t.Cleanup(func() {
		availabilityJSON = origAvailabilityJSON
		availabilityAllClouds = origAvailabilityAllClouds
	})
	availabilityJSON = true
	availabilityAllClouds = true // Include all clouds for this test

	err := runAvailability(availabilityCmd, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "AMPERE_16") {
		t.Errorf("expected AMPERE_16 in output, got: %s", output)
	}
	if !strings.Contains(output, "ADA_LOVELACE_24") {
		t.Errorf("expected ADA_LOVELACE_24 in output, got: %s", output)
	}
}

func TestAvailability_TableOutput(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		getGPUTypesFn: func() ([]pod.GPUType, error) {
			return []pod.GPUType{
				{
					ID:                        "AMPERE_16",
					DisplayName:               "RTX A4000",
					MemoryInGb:                16,
					SecurePrice:               0.44,
					CommunityPrice:            0.22,
					SecureSpotPrice:           0.22,
					CommunitySpotPrice:        0.11,
					SecureCloud:               true,
					CommunityCloud:            true,
					MaxGpuCountSecureCloud:    15,
					MaxGpuCountCommunityCloud: 8,
				},
			}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	origCfgFile := cfgFile
	t.Cleanup(func() { cfgFile = origCfgFile })
	cfgFile = configPath

	var stdout bytes.Buffer
	availabilityCmd.SetOut(&stdout)
	availabilityCmd.SetErr(bytes.NewBuffer(nil))

	// Test with table output (secure only, default)
	origAvailabilityJSON := availabilityJSON
	origAvailabilityAllClouds := availabilityAllClouds
	t.Cleanup(func() {
		availabilityJSON = origAvailabilityJSON
		availabilityAllClouds = origAvailabilityAllClouds
	})
	availabilityJSON = false
	availabilityAllClouds = false // Only secure, default behavior

	err := runAvailability(availabilityCmd, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := stdout.String()
	// Only AMPERE_16 has secure availability, ADA_LOVELACE_24 does not
	if !strings.Contains(output, "AMPERE_16") {
		t.Errorf("expected AMPERE_16 in output, got: %s", output)
	}
	if strings.Contains(output, "ADA_LOVELACE_24") {
		t.Errorf("expected ADA_LOVELACE_24 NOT in output (not secure available), got: %s", output)
	}
	if !strings.Contains(output, "SECURE PRICE") {
		t.Errorf("expected SECURE PRICE header in output, got: %s", output)
	}
}
