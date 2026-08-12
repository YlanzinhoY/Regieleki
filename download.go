package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type downloadProgress struct {
	Downloaded int64
	Total      int64
	Speed      float64
	Mirror     int
}

var selectMirrorStart = func(count int) int {
	return mathrand.IntN(count)
}

type downloadResult struct {
	Path         string
	Downloaded   int64
	Total        int64
	Elapsed      time.Duration
	AverageSpeed float64
	Mirror       int
}

type cdnLimitError struct {
	StatusCode int
	Mirror     int
}

func (err *cdnLimitError) Error() string {
	return fmt.Sprintf("mirror %d refused this request with HTTP %d", err.Mirror, err.StatusCode)
}

type cdnMirrorError struct {
	Mirror int
	Err    error
}

func (err *cdnMirrorError) Error() string {
	return fmt.Sprintf("mirror %d failed: %v", err.Mirror, err.Err)
}

func (err *cdnMirrorError) Unwrap() error {
	return err.Err
}

type progressWriter struct {
	writer     io.Writer
	downloaded int64
	total      int64
	mirror     int
	startedAt  time.Time
	lastReport time.Time
	report     func(downloadProgress)
}

func (writer *progressWriter) Write(chunk []byte) (int, error) {
	bytesWritten, err := writer.writer.Write(chunk)
	writer.downloaded += int64(bytesWritten)

	now := time.Now()
	if writer.lastReport.IsZero() || now.Sub(writer.lastReport) >= 100*time.Millisecond || err != nil {
		writer.emit(now)
	}

	return bytesWritten, err
}

func (writer *progressWriter) emit(now time.Time) {
	elapsed := now.Sub(writer.startedAt)
	speed := float64(writer.downloaded)
	if elapsed > 0 {
		speed /= elapsed.Seconds()
	}
	writer.report(downloadProgress{
		Downloaded: writer.downloaded,
		Total:      writer.total,
		Speed:      speed,
		Mirror:     writer.mirror,
	})
	writer.lastReport = now
}

func downloadFile(
	ctx context.Context,
	conversion Conversion,
	directory string,
	report func(downloadProgress),
) (downloadResult, error) {
	downloadURLs := conversion.DownloadURLs
	if len(downloadURLs) == 0 && conversion.DownloadURL != "" {
		downloadURLs = []string{conversion.DownloadURL}
	}
	if len(downloadURLs) == 0 {
		return downloadResult{}, errors.New("no CDN mirrors configured")
	}
	if report == nil {
		report = func(downloadProgress) {}
	}

	client := &http.Client{}
	var lastError error
	for _, mirrorIndex := range orderedMirrorIndexes(len(downloadURLs)) {
		downloadURL := downloadURLs[mirrorIndex]
		mirrorNumber := workerMirrorStart + mirrorIndex
		report(downloadProgress{Mirror: mirrorNumber})
		result, err := downloadFromURL(ctx, client, conversion, downloadURL, mirrorNumber, directory, report)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return downloadResult{}, ctx.Err()
		}
		lastError = &cdnMirrorError{Mirror: mirrorNumber, Err: err}
	}

	return downloadResult{}, fmt.Errorf("all CDN mirrors failed: %w", lastError)
}

func orderedMirrorIndexes(count int) []int {
	if count <= 0 {
		return nil
	}

	start := selectMirrorStart(count)
	indexes := make([]int, 0, count)
	for offset := 0; offset < count; offset++ {
		indexes = append(indexes, (start+offset)%count)
	}
	return indexes
}

func downloadFromURL(
	ctx context.Context,
	client *http.Client,
	conversion Conversion,
	downloadURL string,
	mirrorNumber int,
	directory string,
	report func(downloadProgress),
) (downloadResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return downloadResult{}, fmt.Errorf("creating request: %w", err)
	}
	request.Header.Set("User-Agent", "regieleki/1.0")

	response, err := client.Do(request)
	if err != nil {
		return downloadResult{}, fmt.Errorf("connecting to worker: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
			return downloadResult{}, &cdnLimitError{StatusCode: response.StatusCode, Mirror: mirrorNumber}
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2048))
		message := compactMessage(string(body))
		if readErr != nil || message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return downloadResult{}, fmt.Errorf("worker returned HTTP %d: %s", response.StatusCode, message)
	}

	filename := responseFilename(response, conversion.FileID)
	file, path, err := createOutputFile(directory, filename)
	if err != nil {
		return downloadResult{}, err
	}

	keepFile := false
	defer func() {
		if !keepFile {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()

	if report == nil {
		report = func(downloadProgress) {}
	}
	startedAt := time.Now()
	writer := &progressWriter{
		writer:    file,
		total:     response.ContentLength,
		mirror:    mirrorNumber,
		startedAt: startedAt,
		report:    report,
	}
	_, err = io.CopyBuffer(writer, response.Body, make([]byte, 64*1024))
	if err != nil {
		return downloadResult{}, fmt.Errorf("downloading file: %w", err)
	}
	writer.emit(time.Now())

	if err := file.Close(); err != nil {
		return downloadResult{}, fmt.Errorf("closing file: %w", err)
	}
	keepFile = true

	elapsed := time.Since(startedAt)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	averageSpeed := float64(writer.downloaded) / elapsed.Seconds()

	return downloadResult{
		Path:         path,
		Downloaded:   writer.downloaded,
		Total:        response.ContentLength,
		Elapsed:      elapsed,
		AverageSpeed: averageSpeed,
		Mirror:       mirrorNumber,
	}, nil
}

func responseFilename(response *http.Response, fileID string) string {
	disposition := response.Header.Get("Content-Disposition")
	if disposition != "" {
		_, parameters, err := mime.ParseMediaType(disposition)
		if err == nil {
			for _, key := range []string{"filename", "filename*"} {
				if name := safeFilename(parameters[key], ""); name != "" {
					return name
				}
			}
		}
	}
	return "file_" + fileID
}

func compactMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 240 {
		return message[:240] + "..."
	}
	return message
}

func safeFilename(name string, fallback string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\x00", "")
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

func createOutputFile(directory string, filename string) (*os.File, string, error) {
	if directory == "" {
		directory = "."
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating output directory: %w", err)
	}

	filename = safeFilename(filename, "download")
	extension := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)

	for attempt := 0; attempt < 10000; attempt++ {
		candidate := filename
		if attempt > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, attempt, extension)
		}
		path := filepath.Join(directory, candidate)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return file, path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", fmt.Errorf("creating output file: %w", err)
		}
	}

	return nil, "", errors.New("could not find an available filename")
}
