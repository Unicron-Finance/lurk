package multi

import (
	"context"

	"github.com/Unicron-Finance/lurk/internal/core"
)

// MultiPlugin provides multi-file tailing capabilities.
type MultiPlugin struct{}

// Name returns the unique identifier for this plugin.
func (p *MultiPlugin) Name() string {
	return "multi"
}

// Init initializes the plugin.
func (p *MultiPlugin) Init() error {
	return nil
}

// Run processes events from the input channel and returns an output channel.
func (p *MultiPlugin) Run(ctx context.Context, input <-chan core.Event) (<-chan core.Event, error) {
	return nil, nil
}

// Close shuts down the plugin.
func (p *MultiPlugin) Close() error {
	return nil
}

// Provides returns the capabilities this plugin provides.
func (p *MultiPlugin) Provides() []string {
	return []string{"multi"}
}

// Requires returns the capabilities this plugin depends on.
func (p *MultiPlugin) Requires() []string {
	return []string{"tail"}
}

// Extends returns the capability this plugin extends.
func (p *MultiPlugin) Extends() string {
	return "tail"
}

func init() {
	core.Register(&MultiPlugin{})
}
