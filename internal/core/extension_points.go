package core

// ExtensionPoints defines the core extension points where plugins can attach.
// These represent the fundamental stages of the event processing pipeline.
var ExtensionPoints = struct {
	Input     string
	Transform string
	Output    string
}{
	Input:     "input",
	Transform: "transform",
	Output:    "output",
}
