package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDownloadDirectory(t *testing.T) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir returned an error: %v", err)
	}

	expected := filepath.Join(homeDirectory, "Downloads")
	if actual := defaultDownloadDirectory(); actual != expected {
		t.Fatalf("expected download directory %q, got %q", expected, actual)
	}
}
