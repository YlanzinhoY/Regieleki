package main

import "testing"

func TestConvertInputAXCPM2gM(t *testing.T) {
	conversion, err := convertInput("AXCPM2gM")
	if err != nil {
		t.Fatalf("convertInput returned an error: %v", err)
	}

	if conversion.FileID != "AXCPM2gM" {
		t.Fatalf("expected file ID AXCPM2gM, got %q", conversion.FileID)
	}

	expectedURL := "https://cdn18.pixeldrain.eu.cc/api/file/AXCPM2gM"
	if conversion.DownloadURL != expectedURL {
		t.Fatalf("expected URL %q, got %q", expectedURL, conversion.DownloadURL)
	}
}

func TestConvertInputExtractsIDFromPixeldrainURL(t *testing.T) {
	conversion, err := convertInput("https://pixeldrain.com/u/AXCPM2gM")
	if err != nil {
		t.Fatalf("convertInput returned an error: %v", err)
	}

	if conversion.FileID != "AXCPM2gM" {
		t.Fatalf("expected file ID AXCPM2gM, got %q", conversion.FileID)
	}
}

func TestConvertInputRejectsInvalidID(t *testing.T) {
	if _, err := convertInput("AXCPM2gM/extra"); err == nil {
		t.Fatal("expected an invalid ID error")
	}
}
