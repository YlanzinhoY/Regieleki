package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
