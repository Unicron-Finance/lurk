package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Unicron-Finance/lurk/internal/core"
	_ "github.com/Unicron-Finance/lurk/internal/tail"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: lurk <file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Get the tail plugin from the registry
	plugin, ok := core.GlobalRegistry.GetPlugin("tail")
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: tail plugin not found in registry")
		os.Exit(1)
	}

	// Configure the plugin with the file path via config
	if configurable, ok := plugin.(interface{ SetConfig(map[string]interface{}) }); ok {
		configurable.SetConfig(map[string]interface{}{"path": filePath})
	}

	// Initialize the plugin
	if err := plugin.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing plugin: %v\n", err)
		os.Exit(1)
	}
	defer plugin.Close()

	// Set up graceful shutdown on Ctrl+C
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the plugin - tail is a source, so input is nil
	events, err := plugin.Run(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running plugin: %v\n", err)
		os.Exit(1)
	}

	// Print each event as it arrives
	for event := range events {
		fmt.Println(string(event.Data))
	}
}
