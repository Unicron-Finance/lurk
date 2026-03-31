package tail

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Line represents a single line read from the tailed file.
type Line struct {
	Text string
	Time time.Time
}

// Tail provides fsnotify-based file watching and tailing capabilities.
type Tail struct {
	path     string
	watcher  *fsnotify.Watcher
	file     *os.File
	reader   *bufio.Reader
	position int64
	started  bool
	mu       sync.Mutex
}

// NewTailer creates a new Tail instance for the given file path.
func NewTailer(path string) (*Tail, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Tail{
		path:    path,
		watcher: watcher,
	}, nil
}

// Start begins watching the file and returns a channel that receives new lines.
// The watch continues until the context is cancelled or an unrecoverable error occurs.
func (t *Tail) Start(ctx context.Context) (<-chan Line, error) {
	lines := make(chan Line, 100) // Buffered channel

	// Open the file and seek to end (tail behavior - start from end)
	if err := t.openFile(); err != nil {
		t.watcher.Close()
		return nil, err
	}

	// Mark as started so subsequent opens know to seek to end
	t.started = true

	// Add the file to the watcher
	if err := t.watcher.Add(t.path); err != nil {
		t.file.Close()
		t.watcher.Close()
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	// Also watch the parent directory to detect file rotation
	dir := getDir(t.path)
	t.watcher.Add(dir)

	go t.run(ctx, lines)

	return lines, nil
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return "./"
}

// openFile opens the file and seeks to the current position.
func (t *Tail) openFile() error {
	file, err := os.Open(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %w", err)
		}
		return fmt.Errorf("failed to open file: %w", err)
	}

	// If first time opening, seek to end (tail behavior)
	// Otherwise seek to tracked position
	if !t.started {
		pos, err := file.Seek(0, io.SeekEnd)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to seek to end of file: %w", err)
		}
		t.position = pos
	} else {
		if _, err := file.Seek(t.position, io.SeekStart); err != nil {
			file.Close()
			return fmt.Errorf("failed to seek in file: %w", err)
		}
	}

	t.file = file
	t.reader = bufio.NewReader(file)
	return nil
}

// readLines reads any new lines from the current file position.
func (t *Tail) readLines(lines chan<- Line) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.file == nil || t.reader == nil {
		return nil
	}

	for {
		line, err := t.reader.ReadString('\n')
		if len(line) > 0 {
			// Update position
			t.position += int64(len(line))

			// Trim trailing newline
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			// Trim trailing carriage return (Windows)
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			// Skip empty lines from trailing newlines
			if line == "" && err == io.EOF {
				return nil
			}

			select {
			case lines <- Line{Text: line, Time: time.Now()}:
			default:
				// Channel full, drop the line
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// handleRotation handles file rotation by reopening the file.
func (t *Tail) handleRotation() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close current file
	if t.file != nil {
		t.file.Close()
		t.file = nil
		t.reader = nil
	}

	// Reset position for new file (start from beginning since it's new)
	t.position = 0

	// Try to open the new file
	file, err := os.Open(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's ok
			return nil
		}
		return err
	}

	t.file = file
	t.reader = bufio.NewReader(file)
	return nil
}

// run is the main event loop that processes fsnotify events and reads lines.
func (t *Tail) run(ctx context.Context, lines chan<- Line) {
	defer close(lines)
	defer t.watcher.Close()
	defer func() {
		t.mu.Lock()
		if t.file != nil {
			t.file.Close()
		}
		t.mu.Unlock()
	}()

	// Initial read (no lines since we started at end)
	t.readLines(lines)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			// Check if event is for our file
			if event.Name != t.path {
				continue
			}

			switch {
			case event.Op&fsnotify.Write == fsnotify.Write:
				// File was written, read new lines
				t.readLines(lines)

			case event.Op&fsnotify.Rename == fsnotify.Rename || event.Op&fsnotify.Remove == fsnotify.Remove:
				// File was renamed or removed (rotation)
				t.handleRotation()

			case event.Op&fsnotify.Create == fsnotify.Create:
				// New file created (could be our file after rotation)
				t.handleRotation()
				t.readLines(lines)
			}

		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			// Log errors but continue watching
			_ = err
		}
	}
}

// Close stops the tailer and cleans up resources.
func (t *Tail) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.file != nil {
		if err := t.file.Close(); err != nil {
			return err
		}
		t.file = nil
	}

	if t.watcher != nil {
		return t.watcher.Close()
	}

	return nil
}
