package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockPlugin is a minimal plugin implementation for testing.
type mockPlugin struct {
	name      string
	provides  []string
	requires  []string
	extends   string
	initErr   error
	runErr    error
	closeErr  error
	initCalled bool
	runCalled  bool
	closeCalled bool
}

func (m *mockPlugin) Name() string                     { return m.name }
func (m *mockPlugin) Init() error                      { m.initCalled = true; return m.initErr }
func (m *mockPlugin) Run(ctx context.Context, input <-chan Event) (<-chan Event, error) {
	m.runCalled = true
	if m.runErr != nil {
		return nil, m.runErr
	}
	out := make(chan Event)
	go func() {
		defer close(out)
		for e := range input {
			out <- e
		}
	}()
	return out, nil
}
func (m *mockPlugin) Close() error                   { m.closeCalled = true; return m.closeErr }
func (m *mockPlugin) Provides() []string             { return m.provides }
func (m *mockPlugin) Requires() []string            { return m.requires }
func (m *mockPlugin) Extends() string                { return m.extends }

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	tail := &mockPlugin{
		name:     "tail",
		provides: []string{"tail", "input"},
		extends:  ExtensionPoints.Input,
	}

	// Register should succeed
	if err := r.Register(tail); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Duplicate registration should fail
	if err := r.Register(tail); err == nil {
		t.Error("Expected error on duplicate registration")
	}

	// Duplicate capability should fail
	tail2 := &mockPlugin{
		name:     "tail2",
		provides: []string{"tail"}, // "tail" already provided
	}
	if err := r.Register(tail2); err == nil {
		t.Error("Expected error on duplicate capability")
	}
}

func TestRegistry_Resolve(t *testing.T) {
	r := NewRegistry()

	// Register tail (base)
	tail := &mockPlugin{
		name:     "tail",
		provides: []string{"tail", "input"},
		extends:  ExtensionPoints.Input,
	}
	if err := r.Register(tail); err != nil {
		t.Fatalf("Register tail failed: %v", err)
	}

	// Register tail.multi (extends tail)
	multi := &mockPlugin{
		name:     "tail.multi",
		provides: []string{"multi-file"},
		requires: []string{"tail"},
		extends:  "tail",
	}
	if err := r.Register(multi); err != nil {
		t.Fatalf("Register multi failed: %v", err)
	}

	// Register tail.multi.filter (extends tail.multi)
	filter := &mockPlugin{
		name:     "tail.multi.filter",
		provides: []string{"filtered"},
		requires: []string{"multi-file"},
		extends:  "multi-file",
	}
	if err := r.Register(filter); err != nil {
		t.Fatalf("Register filter failed: %v", err)
	}

	// Test resolve single capability
	plugins, err := r.Resolve([]string{"tail"})
	if err != nil {
		t.Fatalf("Resolve tail failed: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name() != "tail" {
		t.Errorf("Expected [tail], got %v", pluginNames(plugins))
	}

	// Test resolve with dependency
	plugins, err = r.Resolve([]string{"multi-file"})
	if err != nil {
		t.Fatalf("Resolve multi-file failed: %v", err)
	}
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d: %v", len(plugins), pluginNames(plugins))
	}
	// tail should come before multi
	if plugins[0].Name() != "tail" || plugins[1].Name() != "tail.multi" {
		t.Errorf("Expected order [tail, tail.multi], got %v", pluginNames(plugins))
	}

	// Test resolve deep dependency
	plugins, err = r.Resolve([]string{"filtered"})
	if err != nil {
		t.Fatalf("Resolve filtered failed: %v", err)
	}
	if len(plugins) != 3 {
		t.Errorf("Expected 3 plugins, got %d: %v", len(plugins), pluginNames(plugins))
	}
}

func TestRegistry_Resolve_MissingCapability(t *testing.T) {
	r := NewRegistry()

	_, err := r.Resolve([]string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for missing capability")
	}
}

func TestRegistry_Resolve_MissingDependency(t *testing.T) {
	r := NewRegistry()

	// Register plugin with unmet dependency
	multi := &mockPlugin{
		name:     "tail.multi",
		provides: []string{"multi-file"},
		requires: []string{"tail"}, // tail not registered
	}
	if err := r.Register(multi); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err := r.Resolve([]string{"multi-file"})
	if err == nil {
		t.Error("Expected error for missing dependency")
	}
}

func TestRegistry_Resolve_CircularDependency(t *testing.T) {
	r := NewRegistry()

	// Create circular dependency: A -> B -> A
	a := &mockPlugin{
		name:     "pluginA",
		provides: []string{"capA"},
		requires: []string{"capB"},
	}
	b := &mockPlugin{
		name:     "pluginB",
		provides: []string{"capB"},
		requires: []string{"capA"},
	}

	if err := r.Register(a); err != nil {
		t.Fatalf("Register a failed: %v", err)
	}
	if err := r.Register(b); err != nil {
		t.Fatalf("Register b failed: %v", err)
	}

	_, err := r.Resolve([]string{"capA"})
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

func TestPipeline_Run(t *testing.T) {
	tail := &mockPlugin{
		name:     "tail",
		provides: []string{"tail", "input"},
	}
	multi := &mockPlugin{
		name:     "tail.multi",
		provides: []string{"multi-file"},
		requires: []string{"tail"},
	}

	p := NewPipeline([]Plugin{tail, multi})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use RunAndWait since mock plugins don't generate events
	err := p.RunAndWait(ctx)
	if err != nil {
		t.Errorf("Pipeline run failed: %v", err)
	}

	if !tail.initCalled {
		t.Error("tail Init not called")
	}
	if !tail.runCalled {
		t.Error("tail Run not called")
	}
	if !tail.closeCalled {
		t.Error("tail Close not called")
	}
}

func TestPipeline_InitError(t *testing.T) {
	tail := &mockPlugin{
		name:    "tail",
		initErr: errors.New("init failed"),
	}

	p := NewPipeline([]Plugin{tail})
	_, err := p.Run(context.Background())
	if err == nil {
		t.Error("Expected error from init failure")
	}
}

func pluginNames(plugins []Plugin) []string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Name()
	}
	return names
}
