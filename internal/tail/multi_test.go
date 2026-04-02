package tail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMultiFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create two test files
	file1 := filepath.Join(tmpDir, "file1.log")
	file2 := filepath.Join(tmpDir, "file2.log")

	if err := os.WriteFile(file1, []byte("initial1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("initial2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create multi-file tailer
	tailer, err := NewMultiTailer([]string{file1, file2})
	if err != nil {
		t.Fatalf("NewMultiTailer failed: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lines, err := tailer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write to both files
	time.Sleep(100 * time.Millisecond)

	f1, _ := os.OpenFile(file1, os.O_APPEND|os.O_WRONLY, 0644)
	f1.WriteString("line from file1\n")
	f1.Close()

	f2, _ := os.OpenFile(file2, os.O_APPEND|os.O_WRONLY, 0644)
	f2.WriteString("line from file2\n")
	f2.Close()

	// Collect lines
	var gotLines []string
	done := make(chan struct{})
	go func() {
		for line := range lines {
			gotLines = append(gotLines, line.Text)
			if len(gotLines) >= 2 {
				break
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for lines")
	}

	// Verify format: [filename] content
	foundFile1 := false
	foundFile2 := false
	for _, line := range gotLines {
		if strings.Contains(line, "[file1.log]") && strings.Contains(line, "line from file1") {
			foundFile1 = true
		}
		if strings.Contains(line, "[file2.log]") && strings.Contains(line, "line from file2") {
			foundFile2 = true
		}
	}

	if !foundFile1 {
		t.Errorf("Expected line prefixed with [file1.log], got: %v", gotLines)
	}
	if !foundFile2 {
		t.Errorf("Expected line prefixed with [file2.log], got: %v", gotLines)
	}
}
