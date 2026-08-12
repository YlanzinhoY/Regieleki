package main

import (
	"os"
	"path/filepath"
)

func defaultDownloadDirectory() string {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "Downloads"
	}
	return filepath.Join(homeDirectory, "Downloads")
}
