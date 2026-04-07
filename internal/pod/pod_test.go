package pod

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/romanvolkov/runpod-launcher/internal/config"
)

// mockPodClient implements PodClient for testing without real HTTP calls.
type mockPodClient struct {
	createFn     func(*config.Config, string) (string, error)
	getStatusFn  func(string) (*PodStatus, error)
	terminateFn  func(string) error
	findByNameFn func(string) (string, error)
}

func (m *mockPodClient) CreatePod(cfg *config.Config, llmAPIKey string) (string, error) {
	if m.createFn != nil {
		return m.createFn(cfg, llmAPIKey)
	}
	return "", errors.New("createFn not set")
}

func (m *mockPodClient) GetPodStatus(podID string) (*PodStatus, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(podID)
	}
	return nil, errors.New("getStatusFn not set")
}

func (m *mockPodClient) TerminatePod(podID string) error {
	if m.terminateFn != nil {
		return m.terminateFn(podID)
	}
	return errors.New("terminateFn not set")
}

func (m *mockPodClient) FindPodByName(name string) (string, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(name)
	}
	return "", errors.New("findByNameFn not set")
}

// shortTick is a fast polling interval used by tests to avoid real-time waits.
const shortTick = 10 * time.Millisecond

// TestWaitForReady_SucceedsOnSecondCall verifies that WaitForReady returns nil
// when GetPodStatus returns RUNNING on the second call.
// Uses a 10ms tick interval to avoid a 5-second real-time wait.
func TestWaitForReady_SucceedsOnSecondCall(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount >= 2 {
				return &PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
			}
			return &PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-123", 30*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 GetPodStatus calls, got %d", callCount)
	}
}

// TestWaitForReady_SucceedsImmediately verifies that WaitForReady returns nil
// when the pod is already RUNNING on the first check.
func TestWaitForReady_SucceedsImmediately(t *testing.T) {
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			return &PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-abc", 10*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

// TestWaitForReady_TimesOut verifies that WaitForReady returns a timeout error
// when the pod never becomes RUNNING within the allowed duration.
func TestWaitForReady_TimesOut(t *testing.T) {
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			return &PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	// Use a very short timeout so the test completes quickly.
	err := WaitForReady(mock, "pod-xyz", 1*time.Millisecond, &stderr, shortTick)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention 'timed out', got: %v", err)
	}
}

// TestWaitForReady_PrintsToStderr verifies that progress feedback is written to stderr,
// not stdout, to keep --json output clean.
func TestWaitForReady_PrintsToStderr(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount >= 2 {
				return &PodStatus{ID: podID, Status: "RUNNING"}, nil
			}
			return &PodStatus{ID: podID, Status: "STARTING"}, nil
		},
	}

	var stderr bytes.Buffer
	_ = WaitForReady(mock, "pod-123", 30*time.Second, &stderr, shortTick)

	if stderr.Len() == 0 {
		t.Error("expected progress output on stderr, got none")
	}
}

// TestWaitForReady_ErrorContinues verifies that GetPodStatus errors print "e" to
// stderr and continue polling rather than returning immediately.
func TestWaitForReady_ErrorContinues(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount == 1 {
				// First call (immediate check): not running.
				return &PodStatus{ID: podID, Status: "STARTING"}, nil
			}
			if callCount == 2 {
				// Second call (first tick): return an error.
				return nil, errors.New("transient API error")
			}
			// Third call: success.
			return &PodStatus{ID: podID, Status: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-err", 30*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "e") {
		t.Errorf("expected 'e' in stderr output for error tick, got: %q", stderr.String())
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 GetPodStatus calls, got %d", callCount)
	}
}

// ---- RunPodClient HTTP tests using httptest.NewServer ----

// newTestClient creates a RunPodClient pointed at the given test server URL.
func newTestClient(serverURL string) *RunPodClient {
	return &RunPodClient{
		apiKey:     "test-api-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    serverURL,
	}
}

// TestRunPodClient_CreatePod_BuildsCorrectRequest verifies that CreatePod sends the
// expected GraphQL mutation and parses the returned pod ID.
func TestRunPodClient_CreatePod_BuildsCorrectRequest(t *testing.T) {
	var capturedBody map[string]interface{}
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Respond with a valid pod ID.
		_, _ = w.Write([]byte(`{"data":{"podFindAndDeployOnDemand":{"id":"pod-created-123","desiredStatus":"RUNNING"}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	cfg := &config.Config{
		RunpodAPIKey:    "test-api-key",
		GPUTypeID:       "AMPERE_16",
		ImageName:       "vllm/vllm-openai:latest",
		ModelName:       "mistral/Mistral-7B",
		ContainerDiskGB: 50,
		VolumeMountPath: "/workspace",
	}

	id, err := client.CreatePod(cfg, "test-llm-key")
	if err != nil {
		t.Fatalf("CreatePod returned error: %v", err)
	}
	if id != "pod-created-123" {
		t.Errorf("expected pod ID 'pod-created-123', got %q", id)
	}

	// Authorization header must use Bearer scheme, not query param.
	if capturedAuth != "Bearer test-api-key" {
		t.Errorf("expected Authorization 'Bearer test-api-key', got %q", capturedAuth)
	}

	// Query should reference the expected mutation name.
	query, _ := capturedBody["query"].(string)
	if !strings.Contains(query, "podFindAndDeployOnDemand") {
		t.Errorf("expected mutation podFindAndDeployOnDemand in query, got: %s", query)
	}
}

// TestRunPodClient_GetPodStatus_Running verifies that GetPodStatus correctly maps
// a non-nil runtime + non-EXITED desiredStatus to Status="RUNNING".
func TestRunPodClient_GetPodStatus_Running(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"pod":{"id":"pod-1","desiredStatus":"RUNNING","runtime":{"uptimeInSeconds":120}}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	status, err := client.GetPodStatus("pod-1")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status != "RUNNING" {
		t.Errorf("expected Status=RUNNING, got %q", status.Status)
	}
	if status.DesiredStatus != "RUNNING" {
		t.Errorf("expected DesiredStatus=RUNNING, got %q", status.DesiredStatus)
	}
}

// TestRunPodClient_GetPodStatus_ExitedWithRuntime verifies that a pod with
// desiredStatus=EXITED but a non-nil runtime is NOT reported as RUNNING.
func TestRunPodClient_GetPodStatus_ExitedWithRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"pod":{"id":"pod-2","desiredStatus":"EXITED","runtime":{"uptimeInSeconds":5}}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	status, err := client.GetPodStatus("pod-2")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status == "RUNNING" {
		t.Error("expected Status != RUNNING for EXITED pod with runtime, got RUNNING")
	}
	if status.DesiredStatus != "EXITED" {
		t.Errorf("expected DesiredStatus=EXITED, got %q", status.DesiredStatus)
	}
}

// TestRunPodClient_GetPodStatus_NullPod verifies that a {"data":{"pod":null}} API response
// returns a NOT_FOUND sentinel rather than an error, so callers can distinguish a missing
// pod from a real API failure.
func TestRunPodClient_GetPodStatus_NullPod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"pod":null}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	status, err := client.GetPodStatus("pod-gone")
	if err != nil {
		t.Fatalf("expected nil error for null pod, got: %v", err)
	}
	if status.Status != "NOT_FOUND" {
		t.Errorf("expected Status=NOT_FOUND for null pod, got %q", status.Status)
	}
}

// TestRunPodClient_GetPodStatus_BlankDesiredStatusWithRuntime verifies that a pod whose
// desiredStatus is blank (API parse gap) but has a non-nil runtime is NOT reported as RUNNING.
// Only an explicit desiredStatus="RUNNING" combined with a non-nil runtime counts as RUNNING.
func TestRunPodClient_GetPodStatus_BlankDesiredStatusWithRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// desiredStatus is absent (blank string after JSON decode) but runtime is present.
		_, _ = w.Write([]byte(`{"data":{"pod":{"id":"pod-blank","desiredStatus":"","runtime":{"uptimeInSeconds":10}}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	status, err := client.GetPodStatus("pod-blank")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status == "RUNNING" {
		t.Error("expected Status != RUNNING for blank desiredStatus with runtime, got RUNNING")
	}
}

// TestRunPodClient_GetPodStatus_StartingNoRuntime verifies that a pod with desiredStatus=STARTING
// and nil runtime (still starting) falls back to reporting desiredStatus as Status,
// which is "STARTING". WaitForReady polls until the pod becomes RUNNING.
func TestRunPodClient_GetPodStatus_StartingNoRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Pod is starting: desiredStatus=RUNNING but runtime is null.
		_, _ = w.Write([]byte(`{"data":{"pod":{"id":"pod-3","desiredStatus":"STARTING","runtime":null}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	status, err := client.GetPodStatus("pod-3")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	// With no runtime and desiredStatus=STARTING, status falls back to desiredStatus.
	if status.Status != "STARTING" {
		t.Errorf("expected Status=STARTING when runtime is nil and desiredStatus=STARTING, got %q", status.Status)
	}
}

// TestRunPodClient_FindPodByName_SkipsExited verifies that FindPodByName does not
// return a pod whose desiredStatus is EXITED.
func TestRunPodClient_FindPodByName_SkipsExited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"myself":{"pods":[
			{"id":"pod-old","name":"llm-launcher","desiredStatus":"EXITED"},
			{"id":"pod-new","name":"other-pod","desiredStatus":"RUNNING"}
		]}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("FindPodByName returned error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID for EXITED pod, got %q", id)
	}
}

// TestRunPodClient_FindPodByName_NullPods verifies that a {"data":{"myself":{"pods":null}}}
// API response (valid "no pods" from RunPod) returns ("", nil) rather than an error.
func TestRunPodClient_FindPodByName_NullPods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"myself":{"pods":null}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("expected nil error for null pods, got: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID for null pods, got %q", id)
	}
}

// TestRunPodClient_FindPodByName_ReturnsActive verifies that FindPodByName returns
// the correct ID for a non-EXITED pod.
func TestRunPodClient_FindPodByName_ReturnsActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"myself":{"pods":[
			{"id":"pod-active","name":"llm-launcher","desiredStatus":"RUNNING"}
		]}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("FindPodByName returned error: %v", err)
	}
	if id != "pod-active" {
		t.Errorf("expected pod ID 'pod-active', got %q", id)
	}
}

// TestRunPodClient_TerminatePod_SendsMutation verifies that TerminatePod sends the
// expected GraphQL mutation with the correct pod ID.
func TestRunPodClient_TerminatePod_SendsMutation(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"podTerminate":null}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	if err := client.TerminatePod("pod-terminate-me"); err != nil {
		t.Fatalf("TerminatePod returned error: %v", err)
	}

	query, _ := capturedBody["query"].(string)
	if !strings.Contains(query, "podTerminate") {
		t.Errorf("expected mutation podTerminate in query, got: %s", query)
	}
}

// TestRunPodClient_GraphQLError verifies that a GraphQL-level error is surfaced
// as a Go error rather than silently ignored.
func TestRunPodClient_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetPodStatus("pod-x")
	if err == nil {
		t.Fatal("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "GraphQL error") {
		t.Errorf("expected 'GraphQL error' in error message, got: %v", err)
	}
}

// TestRunPodClient_HTTPError verifies that a non-200 HTTP response is surfaced as an error.
func TestRunPodClient_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.GetPodStatus("pod-x")
	if err == nil {
		t.Fatal("expected error for HTTP 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected HTTP status code in error message, got: %v", err)
	}
}
