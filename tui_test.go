package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelDoesNotRetryAfterCDNLimit(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	appModel.state = stateError
	appModel.conversion = &Conversion{FileID: "test"}
	appModel.downloadError = &cdnLimitError{StatusCode: 403}
	appModel.cdnBlocked = true

	updatedModel, command := appModel.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(*model)
	if command != nil {
		t.Fatal("expected no retry command after a CDN limit response")
	}
	if updated.state != stateInput {
		t.Fatalf("expected input state, got %v", updated.state)
	}
	if updated.conversion != nil {
		t.Fatal("expected the blocked conversion to be cleared")
	}
	if updated.cdnBlocked {
		t.Fatal("expected CDN blocked state to be cleared")
	}
}

func TestModelViewShowsCurrentMirrorOnError(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	appModel.state = stateError
	appModel.mirror = 40
	appModel.downloadError = &cdnMirrorError{
		Mirror: 40,
		Err:    &cdnLimitError{StatusCode: 403, Mirror: 40},
	}

	if view := appModel.View(); !strings.Contains(view, "Download failed on Mirror 40") {
		t.Fatalf("expected mirror number in error view, got %q", view)
	}
}

func TestModelViewShowsVersionAndPasteHint(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	view := appModel.View()

	for _, expected := range []string{"Version " + version, "Paste ID: Ctrl+Shift+V / Ctrl+V"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in view, got %q", expected, view)
		}
	}
}

func TestModelInputShowsBlinkingCursor(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	appModel.input = "AXCPM2gM"
	appModel.cursorVisible = true

	if view := appModel.inputView(); !strings.Contains(view, "▌") {
		t.Fatalf("expected blinking cursor in input view, got %q", view)
	}
}

func TestModelStopsDownloadWithEscape(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	appModel.state = stateDownloading
	appModel.conversion = &Conversion{FileID: "KpQfUiTC"}
	appModel.downloadCancel = func() {}

	updatedModel, command := appModel.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	updated := updatedModel.(*model)
	if command != nil {
		t.Fatal("expected no command when stopping a download")
	}
	if updated.state != stateInput {
		t.Fatalf("expected input state, got %v", updated.state)
	}
	if updated.conversion != nil {
		t.Fatal("expected the conversion to be cleared")
	}
	if updated.downloadCancel != nil {
		t.Fatal("expected the download cancellation function to be cleared")
	}
}

func TestModelViewShowsStopDownloadButton(t *testing.T) {
	appModel := newModel(".", context.Background(), func() {})
	appModel.state = stateDownloading
	appModel.conversion = &Conversion{FileID: "KpQfUiTC"}

	if view := appModel.View(); !strings.Contains(view, "Stop download: Esc / S") {
		t.Fatalf("expected stop button in view, got %q", view)
	}
}
