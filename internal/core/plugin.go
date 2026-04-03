package core

import (
	"context"
	"time"
)

// Event represents a single event flowing through the plugin pipeline.
type Event struct {
	Data      []byte
	Source    string
	Timestamp time.Time
}

// Plugin is the core interface that all plugins must implement.
// Plugins process events through a pipeline: input -> transform -> output.
type Plugin interface {
	// Name returns the unique identifier for this plugin.
	// Examples: "tail", "tail.multi", "tail.multi.filter"
	Name() string

	// Init initializes the plugin before it starts processing events.
	Init() error

	// Run processes events from the input channel and returns an output channel.
	// The plugin reads events from input, processes them, and writes results to the returned channel.
	// If input is nil, the plugin is a source (generates events).
	Run(ctx context.Context, input <-chan Event) (<-chan Event, error)

	// Close shuts down the plugin and releases any resources.
	Close() error
}

// Capable is an optional interface that plugins can implement to declare
type Capable interface {
	// Provides returns the capabilities this plugin provides.
	// Example: []string{"tail", "input"}
	Provides() []string

	// Requires returns the capabilities this plugin depends on.
	// Example: []string{"tail"}
	Requires() []string

	// Extends returns the capability this plugin extends (its parent in the tree).
	// Example: "tail" (for tail.multi extending tail)
	Extends() string
}
