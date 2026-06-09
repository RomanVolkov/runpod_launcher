package graphql

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{"valid key", "test-key", false},
		{"empty key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewClient error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && client == nil {
				t.Fatal("NewClient returned nil client")
			}
		})
	}
}

func TestGetGPUTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"gpuTypes": [
					{
						"id": "A100",
						"displayName": "NVIDIA A100",
						"memoryInGb": 80,
						"secureCloud": true,
						"communityCloud": true
					},
					{
						"id": "A40",
						"displayName": "NVIDIA A40",
						"memoryInGb": 48,
						"secureCloud": false,
						"communityCloud": true
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	gpus, err := client.GetGPUTypes()

	if err != nil {
		t.Fatalf("GetGPUTypes error: %v", err)
	}

	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}

	if gpus[0].ID != "A100" || gpus[0].MemoryInGb != 80 {
		t.Fatalf("unexpected GPU data: %+v", gpus[0])
	}

	if gpus[1].ID != "A40" || gpus[1].DisplayName != "NVIDIA A40" {
		t.Fatalf("unexpected GPU data: %+v", gpus[1])
	}
}

func TestGetLowestPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"lowestPrice": [
					{
						"gpuName": "NVIDIA A100",
						"gpuTypeId": "A100",
						"minimumBidPrice": 0.85,
						"uninterruptablePrice": 1.23,
						"minMemory": 80,
						"minVcpu": 12
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	input := &GPULowestPriceInput{
		GpuCount:      1,
		MinMemoryInGb: 50,
	}
	prices, err := client.GetLowestPrice(input)

	if err != nil {
		t.Fatalf("GetLowestPrice error: %v", err)
	}

	if len(prices) != 1 {
		t.Fatalf("expected 1 price result, got %d", len(prices))
	}

	if prices[0].GPUName != "NVIDIA A100" || prices[0].MinimumBidPrice != 0.85 {
		t.Fatalf("unexpected price data: %+v", prices[0])
	}
}

func TestGetLowestPrice_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"errors": [{"message": "invalid input"}]
		}`))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	input := &GPULowestPriceInput{GpuCount: 1}
	_, err := client.GetLowestPrice(input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "graphql error: invalid input" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuery_NetworkError(t *testing.T) {
	client, _ := NewClient("test-key")
	client.url = "http://invalid-host-that-does-not-exist.invalid"

	input := GraphQLInput{Query: "{ test }"}
	_, err := client.Query(input)

	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestQuery_BadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	input := GraphQLInput{Query: "{ test }"}
	_, err := client.Query(input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "graphql error: status 500: server error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreatePod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": {
				"podFindAndDeployOnDemand": {
					"id": "pod-123",
					"name": "test-pod",
					"desiredStatus": "RUNNING",
					"costPerHr": 0.75,
					"imageName": "vllm/vllm-openai:latest",
					"containerDiskInGb": 50
				}
			}
		}`))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	input := &PodFindAndDeployInput{
		GpuTypeID:         "A100",
		ImageName:         "vllm/vllm-openai:latest",
		Name:              "test-pod",
		ContainerDiskInGb: 50,
		GpuCount:          1,
		StartSsh:          true,
	}

	pod, err := client.CreatePod(input)
	if err != nil {
		t.Fatalf("CreatePod error: %v", err)
	}

	if pod.ID != "pod-123" || pod.Name != "test-pod" {
		t.Fatalf("unexpected pod data: %+v", pod)
	}
}

func TestCreatePod_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"errors": [{"message": "insufficient funds"}]
		}`))
	}))
	defer server.Close()

	client, _ := NewClientWithURL("test-key", server.URL)
	input := &PodFindAndDeployInput{GpuTypeID: "A100"}
	_, err := client.CreatePod(input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "graphql error: insufficient funds" {
		t.Fatalf("unexpected error: %v", err)
	}
}
