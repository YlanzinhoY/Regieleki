package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const workerBaseURL = "https://cdn18.pixeldrain.eu.cc/api/file"

var workerBaseURLs = buildWorkerBaseURLs()

var (
	fileIDFromURLPattern = regexp.MustCompile("(?i)(?:pixeldrain\\.com/u/|pixeldrain\\.eu\\.cc/api/file/)([A-Za-z0-9_]+)")
	fileIDPattern        = regexp.MustCompile("^[A-Za-z0-9_]+$")
)

type Conversion struct {
	FileID       string
	DownloadURL  string
	DownloadURLs []string
}

func convertInput(input string) (Conversion, error) {
	input = normalizeInput(input)
	if input == "" {
		return Conversion{}, errors.New("enter a file ID")
	}

	fileID := input
	if match := fileIDFromURLPattern.FindStringSubmatch(input); len(match) == 2 {
		fileID = match[1]
	}
	if !fileIDPattern.MatchString(fileID) {
		return Conversion{}, errors.New("invalid file ID")
	}

	downloadURLs := make([]string, 0, len(workerBaseURLs))
	for _, baseURL := range workerBaseURLs {
		downloadURLs = append(downloadURLs, fmt.Sprintf("%s/%s", baseURL, fileID))
	}

	return Conversion{
		FileID:       fileID,
		DownloadURL:  downloadURLs[0],
		DownloadURLs: downloadURLs,
	}, nil
}

func buildWorkerBaseURLs() []string {
	baseURLs := make([]string, 0, 33)
	for hostNumber := 18; hostNumber <= 50; hostNumber++ {
		baseURLs = append(baseURLs, fmt.Sprintf("https://cdn%d.pixeldrain.eu.cc/api/file", hostNumber))
	}
	return baseURLs
}

func normalizeInput(input string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) || unicode.IsControl(character) || unicode.Is(unicode.Cf, character) {
			return -1
		}
		return character
	}, input)
}
