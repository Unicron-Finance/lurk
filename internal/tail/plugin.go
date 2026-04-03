package tail

import (
	"context"
	"fmt"

	"github.com/Unicron-Finance/lurk/internal/core"
)

// TailPlugin implements core.Plugin and core.Capable interfaces for tail functionality.
type TailPlugin struct {
	path   string
	tail   *Tail
	config map[string]interface{}
}

// Name returns the unique identifier for this plugin.
func (p *TailPlugin) Name() string {
	return "tail"
}

// Init initializes the plugin before it starts processing events.
// Validates that a file path is configured.
func (p *TailPlugin) Init() error {
	// Check if path was set via config
	if p.config != nil {
		if path, ok := p.config["path"].(string); ok && path != "" {
			p.path = path
		}
	}

	if p.path == "" {
		return fmt.Errorf("tail plugin requires a file path (set via config['path'] or constructor)")
	}

	return nil
}

// Run processes events from the input channel and returns an output channel.
// For tail plugin (a source), input is nil and it generates events from tailed lines.
func (p *TailPlugin) Run(ctx context.Context, input <-chan core.Event) (<-chan core.Event, error) {
	tail, err := NewTailer(p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to create tailer: %w", err)
	}
	p.tail = tail

	lines, err := tail.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start tailer: %w", err)
	}

	events := make(chan core.Event, 100)

	go func() {
		defer close(events)
		for line := range lines {
			event := core.Event{
				Data:      []byte(line.Text),
				Source:    p.path,
				Timestamp: line.Time,
			}
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

// Close shuts down the plugin and releases any resources.
func (p *TailPlugin) Close() error {
	if p.tail != nil {
		return p.tail.Close()
	}
	return nil
}

// Provides returns the capabilities this plugin provides.
func (p *TailPlugin) Provides() []string {
	return []string{"tail", "input"}
}

// Requires returns the capabilities this plugin depends on.
func (p *TailPlugin) Requires() []string {
	return []string{}
}

// Extends returns the capability this plugin extends (its parent in the tree).
func (p *TailPlugin) Extends() string {
	return ""
}

// SetPath sets the file path for the tail plugin (used for configuration).
func (p *TailPlugin) SetPath(path string) {
	p.path = path
}

// SetConfig sets the plugin configuration.
func (p *TailPlugin) SetConfig(config map[string]interface{}) {
	p.config = config
}

func init() {
	core.Register(&TailPlugin{})
}
