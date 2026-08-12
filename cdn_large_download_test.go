package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const (
	largeDownloadTestEnv     = "REGIELEKI_RUN_LARGE_DOWNLOAD_TEST"
	largeDownloadTargetSize  = int64(6_000_000_000)
	largeDownloadMaxRequests = 10
)

func TestCDNAllowsMoreThanSixGB(t *testing.T) {
	if os.Getenv(largeDownloadTestEnv) != "1" {
		t.Skipf("set %s=1 to run the network test; it downloads at least 6 GB", largeDownloadTestEnv)
	}

	conversion, err := convertInput("KpQfUiTC")
	if err != nil {
		t.Fatalf("convertInput returned an error: %v", err)
	}

	client := &http.Client{}
	var totalDownloaded int64
	requestNumber := 0

	for totalDownloaded < largeDownloadTargetSize {
		requestNumber++
		if requestNumber > largeDownloadMaxRequests {
			t.Fatalf("download did not reach 6 GB within %d requests; received %s", largeDownloadMaxRequests, formatBytes(totalDownloaded))
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, conversion.DownloadURL, nil)
		if err != nil {
			t.Fatalf("creating request %d: %v", requestNumber, err)
		}
		request.Header.Set("User-Agent", "regieleki/1.0 large-download-test")

		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("request %d failed after %s: %v", requestNumber, formatBytes(totalDownloaded), err)
		}

		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			response.Body.Close()
			t.Fatalf("request %d returned HTTP %d", requestNumber, response.StatusCode)
		}

		bytesDownloaded, copyErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if copyErr != nil {
			t.Fatalf("request %d failed after receiving %s: %v", requestNumber, formatBytes(totalDownloaded), copyErr)
		}
		if closeErr != nil {
			t.Fatalf("closing response %d: %v", requestNumber, closeErr)
		}
		if bytesDownloaded == 0 {
			t.Fatalf("request %d returned an empty response", requestNumber)
		}

		totalDownloaded += bytesDownloaded
		t.Logf("request %d downloaded %s; cumulative total: %s / %s", requestNumber, formatBytes(bytesDownloaded), formatBytes(totalDownloaded), formatBytes(largeDownloadTargetSize))
	}

	t.Logf("CDN streamed %s successfully across %d request(s) for %s", formatBytes(totalDownloaded), requestNumber, conversion.FileID)
}

func Example_largeDownloadTestCommand() {
	fmt.Println("$env:REGIELEKI_RUN_LARGE_DOWNLOAD_TEST = \"1\"")
	fmt.Println("go test -run '^TestCDNAllowsMoreThanSixGB$' -count=1 -v")
	// Output:
	// $env:REGIELEKI_RUN_LARGE_DOWNLOAD_TEST = "1"
	// go test -run '^TestCDNAllowsMoreThanSixGB$' -count=1 -v
}
