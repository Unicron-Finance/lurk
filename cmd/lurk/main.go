package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Unicron-Finance/lurk/internal/tail"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: lurk <file>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Create tailer
	tailer, err := tail.NewTailer(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer tailer.Close()

	// Set up graceful shutdown on Ctrl+C
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start tailing
	lines, err := tailer.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print each line as it arrives
	for line := range lines {
		fmt.Println(line.Text)
	}
}
