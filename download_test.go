package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFileTreatsCDNLimitResponsesAsBlocked(t *testing.T) {
	for _, statusCode := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.WriteHeader(statusCode)
			}))
			defer server.Close()

			_, err := downloadFile(
				context.Background(),
				Conversion{FileID: "test", DownloadURL: server.URL},
				t.TempDir(),
				nil,
			)
			var limitError *cdnLimitError
			if !errors.As(err, &limitError) {
				t.Fatalf("expected cdnLimitError, got %v", err)
			}
			if limitError.StatusCode != statusCode {
				t.Fatalf("expected status %d, got %d", statusCode, limitError.StatusCode)
			}
		})
	}
}

func TestDownloadFileFallsBackToNextCDNMirror(t *testing.T) {
	previousMirrorStart := selectMirrorStart
	selectMirrorStart = func(int) int { return 0 }
	defer func() { selectMirrorStart = previousMirrorStart }()

	firstMirror := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.WriteHeader(http.StatusForbidden)
	}))
	defer firstMirror.Close()

	secondMirror := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Disposition", `attachment; filename="fallback.bin"`)
		_, _ = responseWriter.Write([]byte("mirror data"))
	}))
	defer secondMirror.Close()

	outputDirectory := t.TempDir()
	result, err := downloadFile(
		context.Background(),
		Conversion{
			FileID:       "test",
			DownloadURLs: []string{firstMirror.URL, secondMirror.URL},
		},
		outputDirectory,
		nil,
	)
	if err != nil {
		t.Fatalf("downloadFile returned an error: %v", err)
	}

	content, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("reading downloaded file %q: %v", result.Path, err)
	}
	if string(content) != "mirror data" {
		t.Fatalf("expected fallback mirror data, got %q", content)
	}
	if filepath.Base(result.Path) != "fallback.bin" {
		t.Fatalf("expected fallback.bin, got %q", filepath.Base(result.Path))
	}
	if result.Downloaded != int64(len("mirror data")) {
		t.Fatalf("expected %d downloaded bytes, got %d", len("mirror data"), result.Downloaded)
	}
	if result.Mirror != 19 {
		t.Fatalf("expected fallback mirror 19, got %d", result.Mirror)
	}
}

func TestOrderedMirrorIndexesStartsAtSelectedMirror(t *testing.T) {
	previousMirrorStart := selectMirrorStart
	selectMirrorStart = func(int) int { return 32 }
	defer func() { selectMirrorStart = previousMirrorStart }()

	indexes := orderedMirrorIndexes(33)
	if len(indexes) != 33 {
		t.Fatalf("expected 33 mirror indexes, got %d", len(indexes))
	}
	if indexes[0] != 32 {
		t.Fatalf("expected the first mirror index to be 32, got %d", indexes[0])
	}
	if indexes[1] != 0 {
		t.Fatalf("expected mirror order to wrap to index 0, got %d", indexes[1])
	}
	if indexes[len(indexes)-1] != 31 {
		t.Fatalf("expected the last mirror index to be 31, got %d", indexes[len(indexes)-1])
	}
}
