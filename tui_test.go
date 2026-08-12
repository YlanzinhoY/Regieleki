package main

import (
	"context"
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
