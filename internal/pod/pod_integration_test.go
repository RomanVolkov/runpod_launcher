package pod

import (
	"testing"
)

// TestGetOllamaModelContext_Gemma4 tests that gemma4:31b returns the maximum context window.
func TestGetOllamaModelContext_Gemma4(t *testing.T) {
	ctx, err := GetOllamaModelContext("gemma4:31b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedGemma4Context = 262144
	if ctx != expectedGemma4Context {
		t.Errorf("gemma4:31b: expected %d tokens, got %d", expectedGemma4Context, ctx)
	}
}

// TestGetOllamaModelContext_Gemma4Tag tests that gemma4:latest also works.
func TestGetOllamaModelContext_Gemma4Tag(t *testing.T) {
	ctx, err := GetOllamaModelContext("gemma4:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedGemma4Context = 262144
	if ctx != expectedGemma4Context {
		t.Errorf("gemma4:latest: expected %d tokens, got %d", expectedGemma4Context, ctx)
	}
}

// TestGetOllamaModelContext_Mistral tests mistral returns correct context.
func TestGetOllamaModelContext_Mistral(t *testing.T) {
	ctx, err := GetOllamaModelContext("mistral")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedMistralContext = 32768
	if ctx != expectedMistralContext {
		t.Errorf("mistral: expected %d tokens, got %d", expectedMistralContext, ctx)
	}
}

// TestGetOllamaModelContext_Llama3 tests Llama 3 returns correct context.
func TestGetOllamaModelContext_Llama3(t *testing.T) {
	ctx, err := GetOllamaModelContext("llama3:8b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedLlama3Context = 8192
	if ctx != expectedLlama3Context {
		t.Errorf("llama3: expected %d tokens, got %d", expectedLlama3Context, ctx)
	}
}

// TestGetOllamaModelContext_Llama31 tests Llama 3.1 returns the maximum context window.
func TestGetOllamaModelContext_Llama31(t *testing.T) {
	ctx, err := GetOllamaModelContext("llama3.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedLlama31Context = 131072
	if ctx != expectedLlama31Context {
		t.Errorf("llama3.1: expected %d tokens, got %d", expectedLlama31Context, ctx)
	}
}

// TestGetOllamaModelContext_Unknown tests that unknown models return 0 gracefully.
func TestGetOllamaModelContext_Unknown(t *testing.T) {
	ctx, err := GetOllamaModelContext("unknown_model:123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx != 0 {
		t.Errorf("unknown model: expected 0, got %d", ctx)
	}
}
