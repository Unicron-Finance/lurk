package tail

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Unicron-Finance/lurk/internal/core"
)

func TestTailPlugin_Registration(t *testing.T) {
	// Test that the plugin is registered in the global registry
	plugin, ok := core.GlobalRegistry.GetPlugin("tail")
	if !ok {
		t.Fatal("TailPlugin should be registered in GlobalRegistry")
	}

	if plugin.Name() != "tail" {
		t.Errorf("Expected plugin name 'tail', got '%s'", plugin.Name())
	}
}

func TestTailPlugin_Capabilities(t *testing.T) {
	plugin := &TailPlugin{}

	// Test Provides
	provides := plugin.Provides()
	expectedProvides := []string{"tail", "input"}
	if len(provides) != len(expectedProvides) {
		t.Errorf("Expected Provides to have %d items, got %d", len(expectedProvides), len(provides))
	}
	for i, cap := range expectedProvides {
		if i >= len(provides) || provides[i] != cap {
			t.Errorf("Expected Provides[%d] to be '%s', got '%s'", i, cap, provides[i])
		}
	}

	// Test Requires
	requires := plugin.Requires()
	if len(requires) != 0 {
		t.Errorf("Expected Requires to be empty, got %v", requires)
	}

	// Test Extends
	extends := plugin.Extends()
	if extends != "" {
		t.Errorf("Expected Extends to be empty string, got '%s'", extends)
	}
}

func TestTailPlugin_Init_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	plugin := &TailPlugin{}
	plugin.SetPath(testFile)

	if err := plugin.Init(); err != nil {
		t.Errorf("Init() should succeed with valid path, got error: %v", err)
	}
}

func TestTailPlugin_Init_MissingPath(t *testing.T) {
	plugin := &TailPlugin{}
	// Don't set path - should fail

	err := plugin.Init()
	if err == nil {
		t.Error("Init() should fail when no path is configured")
	}
}

func TestTailPlugin_Init_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	plugin := &TailPlugin{}
	plugin.SetConfig(map[string]interface{}{"path": testFile})

	if err := plugin.Init(); err != nil {
		t.Errorf("Init() should succeed with path from config, got error: %v", err)
	}
}

func TestTailPlugin_Run_CreatesEvents(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	plugin := &TailPlugin{}
	plugin.SetPath(testFile)

	if err := plugin.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := plugin.Run(ctx, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Give time for watcher to be ready
	time.Sleep(200 * time.Millisecond)

	// Append lines to file
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for append: %v", err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		f.Close()
		t.Fatalf("Failed to write to file: %v", err)
	}
	f.Close()

	// Wait for first event
	var event1 core.Event
	select {
	case event1 = <-events:
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for first event")
	}

	// Write second line
	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for append: %v", err)
	}
	if _, err := f.WriteString("line3\n"); err != nil {
		f.Close()
		t.Fatalf("Failed to write to file: %v", err)
	}
	f.Close()

	// Wait for second event
	var event2 core.Event
	select {
	case event2 = <-events:
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for second event")
	}

	cancel()

	// Verify events
	if string(event1.Data) != "line2" {
		t.Errorf("Expected first event data 'line2', got '%s'", string(event1.Data))
	}
	if event1.Source != testFile {
		t.Errorf("Expected first event source '%s', got '%s'", testFile, event1.Source)
	}
	if event1.Timestamp.IsZero() {
		t.Error("Expected first event to have non-zero timestamp")
	}

	if string(event2.Data) != "line3" {
		t.Errorf("Expected second event data 'line3', got '%s'", string(event2.Data))
	}
	if event2.Source != testFile {
		t.Errorf("Expected second event source '%s', got '%s'", testFile, event2.Source)
	}
	if event2.Timestamp.IsZero() {
		t.Error("Expected second event to have non-zero timestamp")
	}

	// Clean up
	plugin.Close()
}

func TestTailPlugin_Close(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	plugin := &TailPlugin{}
	plugin.SetPath(testFile)

	if err := plugin.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := plugin.Run(ctx, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Give time for tailer to start
	time.Sleep(200 * time.Millisecond)

	// Close should not error
	if err := plugin.Close(); err != nil {
		t.Errorf("Close() should not error, got: %v", err)
	}
}
