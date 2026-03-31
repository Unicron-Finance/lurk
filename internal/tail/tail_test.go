package tail

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTailer(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	if tailer.path != testFile {
		t.Errorf("Expected path %s, got %s", testFile, tailer.path)
	}
}

func TestTail_Start(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give time for watcher to be ready
	time.Sleep(200 * time.Millisecond)

	// Append more lines (tailer should only see these)
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for append: %v", err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		f.Close()
		t.Fatalf("Failed to write to file: %v", err)
	}
	f.Close()

	// Wait for first line
	var line1 Line
	select {
	case line1 = <-lines:
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for first line")
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

	// Wait for second line
	var line2 Line
	select {
	case line2 = <-lines:
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for second line")
	}

	cancel()

	if line1.Text != "line2" {
		t.Errorf("Expected first line 'line2', got '%s'", line1.Text)
	}
	if line2.Text != "line3" {
		t.Errorf("Expected second line 'line3', got '%s'", line2.Text)
	}
}

func TestTail_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.log")

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = tailer.Start(ctx)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestTail_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	// Channel should close after context cancellation
	done := make(chan struct{})
	go func() {
		for range lines {
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - channel closed
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for channel to close after cancellation")
	}
}

func TestTail_FileRotation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte("old content\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for initial setup
	time.Sleep(200 * time.Millisecond)

	// Simulate rotation: rename old file
	oldFile := filepath.Join(tmpDir, "test.log.old")
	if err := os.Rename(testFile, oldFile); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Create new file with new content
	if err := os.WriteFile(testFile, []byte("new content\n"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Append to new file
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open new file: %v", err)
	}
	if _, err := f.WriteString("after rotation\n"); err != nil {
		f.Close()
		t.Fatalf("Failed to write to new file: %v", err)
	}
	f.Close()

	// Wait for first line from new file
	var line1 Line
	select {
	case line1 = <-lines:
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout waiting for first line after rotation")
	}

	// Wait for second line
	var line2 Line
	select {
	case line2 = <-lines:
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout waiting for second line after rotation")
	}

	cancel()

	if line1.Text != "new content" {
		t.Errorf("Expected first line 'new content', got '%s'", line1.Text)
	}
	if line2.Text != "after rotation" {
		t.Errorf("Expected second line 'after rotation', got '%s'", line2.Text)
	}
}

func TestLine_HasTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte("test line\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	f.WriteString("line with timestamp\n")
	f.Close()

	select {
	case line := <-lines:
		if line.Text != "line with timestamp" {
			t.Errorf("Expected 'line with timestamp', got '%s'", line.Text)
		}
		if line.Time.IsZero() {
			t.Error("Expected non-zero timestamp")
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for line")
	}

	cancel()
}

func TestTail_NoNewline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(testFile, []byte("initial\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tailer, err := NewTailer(testFile)
	if err != nil {
		t.Fatalf("NewTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Write without newline - should not be emitted yet
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	f.WriteString("partial")
	f.Close()

	time.Sleep(200 * time.Millisecond)

	// Now add newline
	f, _ = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("\n")
	f.Close()

	select {
	case line := <-lines:
		if line.Text != "partial" {
			t.Errorf("Expected 'partial', got '%s'", line.Text)
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for line")
	}

	cancel()
}
