package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const workerBaseURL = "https://cdn18.pixeldrain.eu.cc/api/file"

var (
	fileIDFromURLPattern = regexp.MustCompile("(?i)(?:pixeldrain\\.com/u/|pixeldrain\\.eu\\.cc/api/file/)([A-Za-z0-9_]+)")
	fileIDPattern        = regexp.MustCompile("^[A-Za-z0-9_]+$")
)

type Conversion struct {
	FileID      string
	DownloadURL string
}

func convertInput(input string) (Conversion, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Conversion{}, errors.New("informe o ID do arquivo")
	}

	fileID := input
	if match := fileIDFromURLPattern.FindStringSubmatch(input); len(match) == 2 {
		fileID = match[1]
	}
	if !fileIDPattern.MatchString(fileID) {
		return Conversion{}, errors.New("ID de arquivo invalido")
	}

	return Conversion{
		FileID:      fileID,
		DownloadURL: fmt.Sprintf("%s/%s", workerBaseURL, fileID),
	}, nil
}
