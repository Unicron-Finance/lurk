package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("lurk - Lightweight log tailer with filtering")
	
	if len(os.Args) < 2 {
		fmt.Println("Usage: lurk <file>")
		os.Exit(1)
	}
	
	filePath := os.Args[1]
	fmt.Printf("Tailing: %s\n", filePath)
}
