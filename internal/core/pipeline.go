package core

import (
	"context"
	"fmt"
	"sync"
)

// Pipeline wires plugins together in dependency order.
// Events flow from the first plugin (input source) through
// transform plugins to the final output plugin.
type Pipeline struct {
	plugins []Plugin
	started bool
	mu      sync.Mutex
}

// NewPipeline creates a new pipeline with the given plugins.
// Plugins must be ordered by dependency (base capabilities first).
func NewPipeline(plugins []Plugin) *Pipeline {
	return &Pipeline{
		plugins: plugins,
	}
}

// Run initializes and runs all plugins in the pipeline.
// Returns the output channel from the final plugin.
func (p *Pipeline) Run(ctx context.Context) (<-chan Event, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil, fmt.Errorf("pipeline already started")
	}

	if len(p.plugins) == 0 {
		return nil, fmt.Errorf("no plugins in pipeline")
	}

	// Initialize all plugins
	for _, plugin := range p.plugins {
		if err := plugin.Init(); err != nil {
			return nil, fmt.Errorf("failed to initialize plugin %q: %w", plugin.Name(), err)
		}
	}

	// Wire plugins together
	var current <-chan Event
	var err error

	for i, plugin := range p.plugins {
		current, err = plugin.Run(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("failed to start plugin %q: %w", plugin.Name(), err)
		}
		_ = i // i used for potential debugging/logging
	}

	p.started = true
	return current, nil
}

// RunAndWait runs the pipeline and waits for the context to be cancelled
// or the output channel to close. Useful for simple cases.
func (p *Pipeline) RunAndWait(ctx context.Context) error {
	out, err := p.Run(ctx)
	if err != nil {
		return err
	}

	// Drain output until done
	for {
		select {
		case <-ctx.Done():
			return p.Close()
		case _, ok := <-out:
			if !ok {
				return p.Close()
			}
		}
	}
}

// Close shuts down all plugins in reverse order.
func (p *Pipeline) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	// Close in reverse order (dependents before dependencies)
	for i := len(p.plugins) - 1; i >= 0; i-- {
		if err := p.plugins[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	p.started = false
	return firstErr
}

// Plugins returns the plugins in the pipeline.
func (p *Pipeline) Plugins() []Plugin {
	return p.plugins
}

// Len returns the number of plugins in the pipeline.
func (p *Pipeline) Len() int {
	return len(p.plugins)
}
