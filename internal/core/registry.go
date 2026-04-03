package core

import (
	"fmt"
	"sort"
)

// Registry manages plugin registration and resolves dependency trees.
type Registry struct {
	plugins   map[string]Plugin          // name -> plugin
	nodes     map[string]*CapabilityNode // name -> node metadata
	provides  map[string]string          // capability -> plugin that provides it
	extPoints map[string][]string        // extension point -> plugins registered there
}

// CapabilityNode holds metadata about a plugin's place in the capability tree.
type CapabilityNode struct {
	Name           string
	ExtensionPoint string
	Provides       []string
	Requires       []string
	Extends        string
	Plugin         Plugin
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:   make(map[string]Plugin),
		nodes:     make(map[string]*CapabilityNode),
		provides:  make(map[string]string),
		extPoints: make(map[string][]string),
	}
}

// Register adds a plugin to the registry.
// Returns an error if a plugin with the same name is already registered.
func (r *Registry) Register(plugin Plugin) error {
	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}

	r.plugins[name] = plugin

	// Extract capability metadata if plugin implements Capable
	node := &CapabilityNode{
		Name:   name,
		Plugin: plugin,
	}

	if capable, ok := plugin.(Capable); ok {
		node.Provides = capable.Provides()
		node.Requires = capable.Requires()
		node.Extends = capable.Extends()

		// Map capabilities to this plugin
		for _, cap := range node.Provides {
			if existing, exists := r.provides[cap]; exists && existing != name {
				return fmt.Errorf("capability %q already provided by %q", cap, existing)
			}
			r.provides[cap] = name
		}

		// Track extension point membership
		if node.Extends != "" {
			r.extPoints[node.Extends] = append(r.extPoints[node.Extends], name)
		}
	}

	r.nodes[name] = node
	return nil
}

// Resolve takes a list of requested capabilities and returns the set of plugins
// needed to satisfy those capabilities, ordered by dependency (leaves first).
// Validates that all requirements are satisfied and detects circular dependencies.
func (r *Registry) Resolve(requested []string) ([]Plugin, error) {
	// Collect all required plugins
	required := make(map[string]bool)
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var resolveCap func(cap string) error
	resolveCap = func(cap string) error {
		// Check for circular dependency
		if recStack[cap] {
			return fmt.Errorf("circular dependency detected involving %q", cap)
		}

		// Already resolved
		if visited[cap] {
			return nil
		}

		recStack[cap] = true
		defer delete(recStack, cap)

		// Find plugin that provides this capability
		pluginName, exists := r.provides[cap]
		if !exists {
			return fmt.Errorf("no plugin provides capability %q", cap)
		}

		// Recursively resolve requirements
		node := r.nodes[pluginName]
		for _, req := range node.Requires {
			if err := resolveCap(req); err != nil {
				return err
			}
		}

		visited[cap] = true
		required[pluginName] = true
		return nil
	}

	// Resolve all requested capabilities
	for _, req := range requested {
		if err := resolveCap(req); err != nil {
			return nil, err
		}
	}

	// Convert required plugins to a slice
	var plugins []Plugin
	for name := range required {
		plugins = append(plugins, r.plugins[name])
	}

	// Sort by dependency order (topological sort)
	// Plugins with fewer dependencies come first
	sort.Slice(plugins, func(i, j int) bool {
		nodeI := r.nodes[plugins[i].Name()]
		nodeJ := r.nodes[plugins[j].Name()]

		// If i requires j, j comes first
		for _, req := range nodeI.Requires {
			if provider, exists := r.provides[req]; exists && provider == plugins[j].Name() {
				return false
			}
		}

		// If j requires i, i comes first
		for _, req := range nodeJ.Requires {
			if provider, exists := r.provides[req]; exists && provider == plugins[i].Name() {
				return true
			}
		}

		// Otherwise sort by name for stability
		return plugins[i].Name() < plugins[j].Name()
	})

	return plugins, nil
}

// BuildPipeline creates a Pipeline from a list of plugins ordered by dependency.
func (r *Registry) BuildPipeline(plugins []Plugin) *Pipeline {
	return NewPipeline(plugins)
}

// GetPlugin returns a plugin by name, or nil if not found.
func (r *Registry) GetPlugin(name string) (Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// ListPlugins returns all registered plugin names.
func (r *Registry) ListPlugins() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GlobalRegistry is the default registry used by the application.
// Plugins can register themselves via init() functions.
var GlobalRegistry = NewRegistry()

// Register adds a plugin to the global registry.
func Register(plugin Plugin) error {
	return GlobalRegistry.Register(plugin)
}
