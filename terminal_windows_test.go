//go:build windows

package main

import (
	"os"
	"testing"
)

func TestIsInteractiveInvocation(t *testing.T) {
	tests := []struct {
		name        string
		arguments   []string
		interactive bool
	}{
		{name: "default TUI", arguments: nil, interactive: true},
		{name: "output directory", arguments: []string{"--output-dir", "downloads"}, interactive: true},
		{name: "convert without ID", arguments: []string{"convert"}, interactive: true},
		{name: "convert with output directory", arguments: []string{"convert", "-o", "downloads"}, interactive: true},
		{name: "convert with ID", arguments: []string{"convert", "AXCPM2gM"}, interactive: false},
		{name: "version", arguments: []string{"--version"}, interactive: false},
		{name: "help", arguments: []string{"help"}, interactive: false},
	}

	originalArguments := os.Args
	defer func() { os.Args = originalArguments }()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Args = append([]string{"regieleki"}, test.arguments...)
			if actual := isInteractiveInvocation(); actual != test.interactive {
				t.Fatalf("expected interactive=%t for %v, got %t", test.interactive, test.arguments, actual)
			}
		})
	}
}
