package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// executeInitDirect calls runInit directly, overriding config.DefaultPath via a temp dir.
// It patches the initForce flag and redirects stdout.
func executeInitDirect(t *testing.T, targetPath string, force bool) (string, error) {
	t.Helper()

	// Temporarily patch DefaultPath by overriding the initCmd path resolution.
	// We achieve this by setting cfgFile to an empty string and using a monkey-patch
	// approach: replace os.UserHomeDir is not feasible, so we use a wrapper function.
	// Instead, we directly call the internal logic with an injectable path.
	origInitForce := initForce
	t.Cleanup(func() { initForce = origInitForce })
	initForce = force

	var stdout bytes.Buffer
	initCmd.SetOut(&stdout)
	initCmd.SetErr(bytes.NewBuffer(nil))

	// We test runInitWithPath which accepts an explicit path, decoupling from $HOME.
	err := runInitWithPath(initCmd, targetPath, force)
	return stdout.String(), err
}

// TestInit_CreatesFile verifies that init creates the config file with 0600 permissions.
func TestInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.toml")

	stdout, err := executeInitDirect(t, path, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected config file to exist at %s, got: %v", path, statErr)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %04o", info.Mode().Perm())
	}

	if !strings.Contains(stdout, path) {
		t.Errorf("expected path in output, got: %s", stdout)
	}
}

// TestInit_ErrorsWhenFileExists verifies that init returns an error if the file
// already exists and --force is not set.
func TestInit_ErrorsWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Create the file first.
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := executeInitDirect(t, path, false)
	if err == nil {
		t.Fatal("expected an error when file exists without --force, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

// TestInit_ForceOverwrites verifies that init --force overwrites an existing file.
func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Create the file with old content.
	if err := os.WriteFile(path, []byte("old content"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := executeInitDirect(t, path, true)
	if err != nil {
		t.Fatalf("expected no error with --force, got: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) == "old content" {
		t.Error("expected file to be overwritten, but it still has old content")
	}
}
